// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	stdErrors "errors"
	"fmt"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fatih/color"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

type renderedError struct {
	err error
}

func (e renderedError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e renderedError) Unwrap() error {
	return e.err
}

var (
	log = logprinter.NewLogger("")
)

func attachUIOutput(ui *progressv2.UI) (restore func()) {
	if ui == nil {
		return func() {}
	}

	oldStdout := tuiv2output.Stdout.Get()
	oldStderr := tuiv2output.Stderr.Get()
	oldColorOut := color.Output
	oldColorErr := color.Error
	oldNoColor := color.NoColor
	oldColorstrEnabled := colorstr.ColorEnabled()

	w := ui.Writer()

	// Keep all user-facing output consistent with the resolved output mode.
	colorEnabled := tuiterm.Resolve(w).Color
	color.NoColor = !colorEnabled
	colorstr.SetColorEnabled(colorEnabled)

	tuiv2output.Stdout.Set(w)
	tuiv2output.Stderr.Set(w)
	logprinter.SetStdout(w)
	logprinter.SetStderr(w)
	color.Output = w
	color.Error = w

	return func() {
		tuiv2output.Stdout.Set(oldStdout)
		tuiv2output.Stderr.Set(oldStderr)
		logprinter.SetStdout(oldStdout)
		logprinter.SetStderr(oldStderr)
		color.Output = oldColorOut
		color.Error = oldColorErr
		color.NoColor = oldNoColor
		colorstr.SetColorEnabled(oldColorstrEnabled)
	}
}

func printInterrupt(ui *progressv2.UI, sig syscall.Signal) {
	if ui == nil {
		return
	}

	msg := fmt.Sprintf("Playground receive signal: %s", sig)
	colorstr.Fprintf(ui.Writer(), "[red][bold]%s[reset]\n", msg)
}

func startPlaygroundSignalHandler(p *Playground, cancelBoot context.CancelCauseFunc, booted, sigReceived *uint32) {
	if p == nil {
		return
	}

	sc := make(chan os.Signal, 2)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	go func() {
		defer signal.Stop(sc)

		stopRequested := false
		for {
			sig := (<-sc).(syscall.Signal)
			if sigReceived != nil {
				atomic.StoreUint32(sigReceived, 1)
			}

			// If bootCluster is not done we just cancel context to make it
			// clean up and return ASAP (including interrupting downloads).
			if booted != nil && cancelBoot != nil && atomic.LoadUint32(booted) == 0 {
				cancelBoot(nil)
			}

			// If we're already stopping (either due to a previous signal or a
			// non-signal stop request), treat subsequent signals as a force kill
			// request.
			if stopRequested || p.Stopping() {
				p.requestForceKill()
				continue
			}
			stopRequested = true
			p.requestStopSignal(sig)
		}
	}()
}

func execute(state *cliState) error {
	if state == nil {
		state = newCLIState()
	}

	arg0 := playgroundCLIArg0()

	rootCmd := &cobra.Command{
		Use: fmt.Sprintf("%s [version]", filepath.Base(os.Args[0])),
		Long: colorstr.Sprintf(`>_ [bold]TiUP Playground[reset] [dim](ng)[reset]

Start and manage a TiDB cluster locally for development.

[bold]Examples:[reset]

  [dim]Start a cluster using latest release version:[reset]
  [cyan]%[1]s[reset]

  [dim]Start a nightly cluster:[reset]
  [cyan]%[1]s nightly[reset]

  [dim]Start a TiKV-only cluster:[reset]
  [cyan]%[1]s --mode tikv-slim[reset]

  [dim]Start a cluster and run in background:[reset]
  [cyan]%[1]s -d[reset]

  [dim]Start a tagged cluster (data will not be cleaned after exit):[reset]
  [cyan]%[1]s --tag foo[reset]

  [dim]Stop the specified cluster:[reset]
  [cyan]%[1]s stop --tag foo[reset]`, arg0),
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiUPVersion().String(),
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			state.tiupDataDir = os.Getenv(localdata.EnvNameInstanceDataDir)
			tiupHome := os.Getenv(localdata.EnvNameHome)
			if tiupHome == "" {
				tiupHome, _ = getAbsolutePath(filepath.Join("~", localdata.ProfileDirName))
			}

			isRoot := cmd.Parent() == nil
			tagExplicit := false
			if f := cmd.Flags().Lookup("tag"); f != nil {
				tagExplicit = f.Changed
			}
			if !isRoot {
				dataParent := filepath.Join(tiupHome, localdata.DataParentDir)
				if shouldIgnoreSubcommandInstanceDataDir(state.tiupDataDir, dataParent) {
					state.tiupDataDir = ""
				}
			}
			state.destroyDataAfterExit = shouldDestroyDataAfterExit(isRoot, state, tagExplicit, tiupHome)

			// For dry-run, prefer stable default paths so the plan output is
			// deterministic when users don't specify a tag.
			if isRoot && state.dryRun && state.tag == "" && state.tiupDataDir == "" {
				state.tag = "dry-run"
				state.dataDir = filepath.Join(tiupHome, localdata.DataParentDir, state.tag)
			} else if isRoot && (state.background || state.runAsDaemon) {
				// In daemon mode, the data directory must not depend on
				// TIUP_INSTANCE_DATA_DIR (it may be cleaned by the TiUP runner when the
				// starter exits).
				if state.tag == "" {
					state.tag = utils.Base62Tag()
				}
				state.dataDir = filepath.Join(tiupHome, localdata.DataParentDir, state.tag)
			} else {
				switch {
				case state.tag != "":
					state.dataDir = filepath.Join(tiupHome, localdata.DataParentDir, state.tag)
				case state.tiupDataDir != "":
					state.dataDir = state.tiupDataDir
					state.tag = filepath.Base(state.dataDir)
				default:
					if isRoot {
						state.tag = utils.Base62Tag()
						state.dataDir = filepath.Join(tiupHome, localdata.DataParentDir, state.tag)
					} else {
						state.dataDir = filepath.Join(tiupHome, localdata.DataParentDir)
					}
				}
			}

			if isRoot {
				if !state.dryRun {
					err := utils.MkdirAll(state.dataDir, 0755)
					if err != nil {
						return err
					}
					if out := tuiv2output.Stdout.Get(); tuiterm.Resolve(out).Control {
						_, _ = fmt.Fprintf(out, "\033]0;TiUP Playground-NG: %s\a", state.tag)
					}
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if state.background && !state.runAsDaemon {
				return runBackgroundStarter(state)
			}

			if len(args) > 0 {
				state.options.Version = args[0]
			} else if state.options.ShOpt.Mode == proc.ModeNextGen {
				state.options.Version = fmt.Sprintf("%s-%s", utils.LatestVersionAlias, utils.NextgenVersionAlias)
			}

			if err := populateDefaultOpt(cmd.Flags(), &state.options); err != nil {
				return err
			}

			if state.dryRun {
				if err := normalizeBootOptionPaths(&state.options); err != nil {
					return err
				}
				if err := ValidateBootOptionsPure(&state.options); err != nil {
					return err
				}

				env, err := environment.InitEnv(repository.Options{}, repository.MirrorOptions{})
				if err != nil {
					return err
				}
				environment.SetGlobalEnv(env)

				plan, err := BuildBootPlan(&state.options, bootPlannerConfig{
					dataDir:            state.dataDir,
					portConflictPolicy: PortConflictNone,
					componentSource:    newEnvComponentSource(env),
				})
				if err != nil {
					return err
				}
				return writeDryRun(tuiv2output.Stdout.Get(), plan, state.dryRunOutput)
			}

			port := utils.MustGetFreePort("127.0.0.1", 9527, state.options.ShOpt.PortOffset)
			releasePID, err := claimPlaygroundPIDFile(state.dataDir, state.tag)
			if err != nil {
				return err
			}
			defer releasePID()

			p := NewPlayground(state.dataDir, port)
			p.destroyDataAfterExit = state.destroyDataAfterExit

			var eventLog *os.File
			if state.runAsDaemon {
				path := filepath.Join(state.dataDir, playgroundTUIEventLogName)
				f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					return err
				}
				eventLog = f
				defer func() { _ = f.Close() }()
			}

			ui := progressv2.New(progressv2.Options{
				Mode:     progressv2.ModeAuto,
				Out:      os.Stderr,
				EventLog: eventLog,
			})
			defer ui.Close()
			p.ui = ui
			p.downloadGroup = ui.Group("Download components")
			p.downloadGroup.SetHideDetailsOnSuccess(true)
			p.downloadGroup.SetSortTasksByTitle(true)
			p.startingGroup = ui.Group("Start instances")
			downloadGroup := p.downloadGroup
			restore := attachUIOutput(ui)
			defer restore()

			var (
				booted      uint32
				sigReceived uint32
			)
			ctx, cancel := context.WithCancelCause(context.Background())
			ctx = context.WithValue(ctx, logprinter.ContextKeyLogger, log)
			defer cancel(nil)
			p.bootCancel = cancel

			downloadProgress := newRepoDownloadProgress(ctx, downloadGroup)
			if rp, ok := downloadProgress.(*repoDownloadProgress); ok {
				p.downloadProgress = rp
			}

			env, err := environment.InitEnv(repository.Options{}, repository.MirrorOptions{
				Context:  ctx,
				Progress: downloadProgress,
			})
			if err != nil {
				return err
			}
			environment.SetGlobalEnv(env)

			startPlaygroundSignalHandler(p, cancel, &booted, &sigReceived)

			bootErr := p.bootCluster(ctx, &state.options)
			if bootErr != nil {
				// Ctrl+C during boot is not a "failure" from user perspective.
				// The signal handler already started shutdown; wait for it to finish.
				if ctx.Err() == context.Canceled && atomic.LoadUint32(&sigReceived) != 0 {
					_ = p.wait()
					return nil
				}

				var rendered renderedError
				alreadyRendered := stdErrors.As(bootErr, &rendered)
				if !alreadyRendered {
					// Freeze the current progress groups into the immutable history area
					// first, so the callout appears after the boot progress snapshot.
					p.abandonActiveGroups()

					// Print an error callout before shutdown output.
					out := p.terminalWriter()

					if p.ui != nil {
						p.ui.PrintLines([]string{""})
					} else {
						fmt.Fprintln(out)
					}
					fmt.Fprint(out, tuiv2output.Callout{
						Style:   tuiv2output.CalloutFailed,
						Content: fmt.Sprintf("Start cluster failed: %v", bootErr),
					}.Render(out))

					bootErr = renderedError{err: fmt.Errorf("Start cluster failed: %w", bootErr)}
				}

				// On boot failure, prefer a graceful shutdown so the terminal output
				// stays consistent with Ctrl+C handling.
				p.requestStopInternal()
				_ = p.wait()
				return bootErr
			}

			atomic.StoreUint32(&booted, 1)

			waitErr := p.wait()
			if waitErr != nil {
				return waitErr
			}

			return nil
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			if env := environment.GlobalEnv(); env != nil {
				return env.Close()
			}
			return nil
		},
	}

	// Cobra's help template uses color escape sequences computed at registration
	// time. Make sure it follows the same progress mode decision as the rest of
	// the playground output.
	if !tuiterm.ResolveFile(os.Stdout).Color || !tuiterm.ResolveFile(os.Stderr).Color {
		color.NoColor = true
	}

	cobra.AddTemplateFunc("pgCmdLine", func(useLine string) string {
		return rewriteCobraUseLine(arg0, useLine)
	})
	cobra.AddTemplateFunc("pgCmdPath", func(commandPath string) string {
		return rewriteCobraCommandPath(arg0, commandPath)
	})

	usageTpl := rootCmd.UsageTemplate()
	usageTpl = strings.ReplaceAll(usageTpl, "{{.UseLine}}", "{{pgCmdLine .UseLine}}")
	usageTpl = strings.ReplaceAll(usageTpl, "{{.CommandPath}}", "{{pgCmdPath .CommandPath}}")
	rootCmd.SetUsageTemplate(usageTpl)

	cc.Init(&cc.Config{
		RootCmd:  rootCmd,
		Headings: cc.Bold,
		Commands: cc.Cyan + cc.Bold,
	})

	usageTpl = rootCmd.UsageTemplate()
	usageTpl = strings.ReplaceAll(usageTpl, "(CommandStyle .CommandPath)", "(CommandStyle (pgCmdPath .CommandPath))")
	rootCmd.SetUsageTemplate(usageTpl)

	// Cobra's default version flag uses the command name derived from `Use`,
	// which is the binary name (e.g. tiup-playground-ng). For standalone runs we
	// want argv0 (e.g. bin/tiup-playground-ng); for TiUP component mode we want
	// `tiup playground-ng[:<ver>]`.
	rootCmd.InitDefaultHelpFlag()
	if f := rootCmd.Flags().Lookup("help"); f != nil {
		f.Usage = fmt.Sprintf("help for %s", arg0)
	}

	rootCmd.InitDefaultVersionFlag()
	if f := rootCmd.Flags().Lookup("version"); f != nil {
		f.Usage = fmt.Sprintf("version for %s", arg0)
	}

	rootCmd.Flags().StringVar(&state.options.ShOpt.Mode, "mode", "tidb", fmt.Sprintf("%s mode: '%s', '%s', '%s', '%s', '%s'", arg0, proc.ModeNormal, proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg, proc.ModeTiKVSlim))
	rootCmd.Flags().StringVar(&state.options.ShOpt.PDMode, "pd.mode", "pd", "PD mode: 'pd', 'ms'")
	rootCmd.Flags().StringVar(&state.options.ShOpt.CSE.S3Endpoint, "cse.s3_endpoint", "http://127.0.0.1:9000",
		fmt.Sprintf("Object store URL for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().StringVar(&state.options.ShOpt.CSE.Bucket, "cse.bucket", "tiflash",
		fmt.Sprintf("Object store bucket for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().StringVar(&state.options.ShOpt.CSE.AccessKey, "cse.access_key", "minioadmin",
		fmt.Sprintf("Object store access key for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().StringVar(&state.options.ShOpt.CSE.SecretKey, "cse.secret_key", "minioadmin",
		fmt.Sprintf("Object store secret key for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().BoolVar(&state.options.ShOpt.HighPerf, "perf", false, "Tune default config for better performance instead of debug troubleshooting")
	rootCmd.Flags().BoolVar(&state.options.ShOpt.EnableTiKVColumnar, "kv.columnar", false,
		fmt.Sprintf("Enable TiKV columnar storage engine, only available when --mode=%s", proc.ModeCSE))
	rootCmd.Flags().BoolVar(&state.options.ShOpt.ForcePull, "force-pull", false, "Force redownload the component. It is useful to manually refresh nightly or broken binaries")
	rootCmd.Flags().BoolVar(&state.dryRun, "dry-run", false, "Only generate the boot plan and exit")
	rootCmd.Flags().StringVar(&state.dryRunOutput, "dry-run-output", "text", "Dry-run output format: text|json")
	rootCmd.Flags().BoolVarP(&state.background, "background", "d", false, "Start playground-ng in background (daemon mode)")
	rootCmd.Flags().BoolVar(&state.runAsDaemon, "run-as-daemon", false, "INTERNAL: run as daemon")
	_ = rootCmd.Flags().MarkHidden("run-as-daemon")

	rootCmd.PersistentFlags().StringVarP(&state.tag, "tag", "T", "", "Specify a tag for playground, data dir of this tag will not be removed after exit")
	rootCmd.Flags().Bool("without-monitor", false, "Don't start prometheus and grafana component")
	rootCmd.Flags().IntVar(&state.options.GrafanaPort, "grafana.port", 3000, "grafana port. If not provided, grafana will use 3000 as its port.")
	rootCmd.Flags().IntVar(&state.options.ShOpt.PortOffset, "port-offset", 0, "If specified, all components will use default_port+port_offset as the port. This argument is useful when you want to start multiple playgrounds on the same host. Recommend to set to 10000, 20000, etc.")

	// NOTE: Do not set default values if they may be changed in different modes.

	registerServiceFlags(rootCmd.Flags(), &state.options)

	rootCmd.Flags().StringVar(&state.options.Host, "host", "127.0.0.1", "Playground cluster host")

	rootCmd.AddCommand(newDisplay(state))
	rootCmd.AddCommand(newScaleOut(state))
	rootCmd.AddCommand(newScaleIn(state))
	rootCmd.AddCommand(newStop(state))
	rootCmd.AddCommand(newStopAll(state))
	rootCmd.AddCommand(newPS(state))

	return rootCmd.Execute()
}

func populateDefaultOpt(flagSet *pflag.FlagSet, options *BootOptions) error {
	if flagSet.Lookup("without-monitor").Changed {
		v, _ := flagSet.GetBool("without-monitor")
		if options != nil {
			options.Monitor = !v
		}
	}

	return applyServiceDefaults(flagSet, options)
}

// getAbsolutePath returns the absolute path
func getAbsolutePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "~/") {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		if wd == "" {
			return "", errors.New("playground running at non-tiup mode")
		}
		path = filepath.Join(wd, path)
	}

	if strings.HasPrefix(path, "~/") {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", errors.Annotatef(err, "retrieve user home failed")
		}
		path = filepath.Join(homedir, path[2:])
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", errors.AddStack(err)
	}

	return absPath, nil
}

func dumpPort(fname string, port int) error {
	return utils.WriteFile(fname, []byte(strconv.Itoa(port)), 0o644)
}

func loadPort(dir string) (port int, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "port"))
	if err != nil {
		return 0, err
	}

	port, err = strconv.Atoi(strings.TrimSpace(string(data)))
	return
}

func shouldIgnoreSubcommandInstanceDataDir(instanceDir, dataParentDir string) bool {
	instanceDir = strings.TrimSpace(instanceDir)
	dataParentDir = strings.TrimSpace(dataParentDir)
	if instanceDir == "" || dataParentDir == "" {
		return false
	}

	instanceDir = filepath.Clean(instanceDir)
	dataParentDir = filepath.Clean(dataParentDir)

	// Only ignore paths under the default TiUP data directory to avoid surprising
	// users who explicitly set TIUP_INSTANCE_DATA_DIR to a custom location.
	sep := string(os.PathSeparator)
	if instanceDir == dataParentDir || !strings.HasPrefix(instanceDir, dataParentDir+sep) {
		return false
	}

	// When users run subcommands (e.g. `tiup playground-ng display`) without a
	// global --tag, the TiUP runner generates a temporary tag and sets
	// TIUP_INSTANCE_DATA_DIR to an empty directory for that invocation. Treat
	// such directories as non-explicit targets so playground-ng can auto-discover
	// running instances under $TIUP_HOME/data.
	tag := filepath.Base(instanceDir)
	if len(tag) < 7 {
		return false
	}
	for _, r := range tag {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		default:
			return false
		}
	}

	entries, err := os.ReadDir(instanceDir)
	if err != nil {
		return false
	}
	for _, ent := range entries {
		name := ent.Name()
		// TiUP runner may write the process meta file into TIUP_INSTANCE_DATA_DIR
		// before the component inspects the directory. Ignore it so subcommands
		// without an explicit --tag can still auto-discover running instances.
		if name == "" || name == ".DS_Store" || name == localdata.MetaFilename {
			continue
		}
		return false
	}

	return true
}

func shouldDestroyDataAfterExit(isRoot bool, state *cliState, tagExplicit bool, tiupHome string) bool {
	if state == nil || !isRoot || state.dryRun || state.background || state.runAsDaemon || tagExplicit {
		return false
	}
	if state.tiupDataDir == "" {
		return true
	}

	instanceDir := filepath.Clean(strings.TrimSpace(state.tiupDataDir))
	dataParent := filepath.Clean(filepath.Join(tiupHome, localdata.DataParentDir))
	sep := string(os.PathSeparator)
	return instanceDir != "" &&
		dataParent != "" &&
		instanceDir != dataParent &&
		strings.HasPrefix(instanceDir, dataParent+sep)
}

func dumpDSN(fname string, dbs []*proc.TiDBInstance, tdbs []*proc.TiProxyInstance) {
	var dsn []string
	for _, db := range dbs {
		dsn = append(dsn, fmt.Sprintf("mysql://root@%s", db.Addr()))
	}
	for _, tdb := range tdbs {
		dsn = append(dsn, fmt.Sprintf("mysql://root@%s", tdb.Addr()))
	}
	_ = utils.WriteFile(fname, []byte(strings.Join(dsn, "\n")), 0o644)
}

func newEtcdClient(endpoint string) (*clientv3.Client, error) {
	// Because etcd client does not support setting logger directly,
	// the configuration of pingcap/log is copied here.
	zapCfg := zap.NewProductionConfig()
	zapCfg.OutputPaths = []string{"stderr"}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
		LogConfig:   &zapCfg,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func newRepoDownloadProgress(ctx context.Context, g *progressv2.Group) repository.DownloadProgress {
	return &repoDownloadProgress{
		ctx:   ctx,
		group: g,
		now:   time.Now,
	}
}

// repoDownloadProgress adapts repository download callbacks into the unified
// progress UI used by playground.
//
// It intentionally lives in playground (not tuiv2) so tuiv2 stays free of any
// repository-specific conventions (like tarball naming rules).
type repoDownloadProgress struct {
	ctx   context.Context
	group *progressv2.Group

	mu   sync.Mutex
	task *progressv2.Task

	expected map[string]*progressv2.Task
	byURL    map[string]*progressv2.Task

	now func() time.Time

	lastUpdateAt time.Time
	lastSize     int64
	latestSize   int64
}

func (p *repoDownloadProgress) SetExpectedDownloads(downloads []DownloadPlan) {
	if p == nil || p.group == nil {
		return
	}

	expected := make(map[string]*progressv2.Task, len(downloads))
	for _, d := range downloads {
		componentID := strings.TrimSpace(d.ComponentID)
		resolved := strings.TrimSpace(d.ResolvedVersion)
		if componentID == "" || resolved == "" {
			continue
		}

		key := componentID + "@" + resolved
		if expected[key] != nil {
			continue
		}

		title := proc.ComponentDisplayName(proc.RepoComponentID(componentID))
		t := p.group.TaskPending(title)
		t.SetMeta(resolved)
		t.SetKindDownload()
		expected[key] = t
	}

	p.mu.Lock()
	p.expected = expected
	p.mu.Unlock()
}

func (p *repoDownloadProgress) Start(rawURL string, size int64) {
	if p == nil || p.group == nil {
		return
	}

	base := downloadTitle(rawURL)
	componentID, resolved, ok := parseComponentVersionFromTarball(base)

	var t *progressv2.Task
	if ok {
		key := componentID + "@" + resolved
		p.mu.Lock()
		t = p.expected[key]
		p.mu.Unlock()
	}

	name, version := downloadDisplay(rawURL)
	if t == nil {
		t = p.group.Task(name)
		if version != "" {
			t.SetMeta(version)
		}
	} else {
		if resolved != "" {
			t.SetMeta(resolved)
		} else if version != "" {
			t.SetMeta(version)
		}
	}
	t.SetMessage("")

	if size > 0 {
		t.SetTotal(size)
	}
	// Set the kind before Start so plain mode prints the download start line
	// even when the task was pre-created as pending.
	t.SetKindDownload()
	t.Start()

	p.mu.Lock()
	if p.byURL == nil {
		p.byURL = make(map[string]*progressv2.Task)
	}
	if rawURL != "" && t != nil {
		p.byURL[rawURL] = t
	}
	p.task = t
	p.lastUpdateAt = time.Time{}
	p.lastSize = 0
	p.latestSize = 0
	p.mu.Unlock()
}

func (p *repoDownloadProgress) SetCurrent(size int64) {
	p.mu.Lock()
	t := p.task
	if t == nil {
		p.mu.Unlock()
		return
	}

	// Repository download callbacks can be very frequent. Throttle SetCurrent
	// updates to avoid flooding the progress event channel / renderer.
	//
	// 10 FPS is enough for a smooth progress UI (TTY redraw is capped anyway) and
	// also keeps daemon-mode event logs at a reasonable size.
	now := p.now()
	const minInterval = 100 * time.Millisecond

	p.latestSize = size
	shouldUpdate := false
	if size < p.lastSize {
		shouldUpdate = true
	} else if p.lastUpdateAt.IsZero() || now.Sub(p.lastUpdateAt) >= minInterval {
		shouldUpdate = true
	}
	if shouldUpdate {
		p.lastUpdateAt = now
		p.lastSize = size
	}
	p.mu.Unlock()

	if !shouldUpdate {
		return
	}
	t.SetCurrent(size)
}

func (p *repoDownloadProgress) Finish() {
	p.mu.Lock()
	t := p.task
	latestSize := p.latestSize
	flush := t != nil && latestSize != p.lastSize
	if flush {
		p.lastUpdateAt = p.now()
		p.lastSize = latestSize
	}
	p.task = nil
	ctx := p.ctx
	p.mu.Unlock()

	if t == nil {
		return
	}

	// Ensure the final progress current is emitted even when the last callback
	// falls into the throttle window.
	if flush {
		t.SetCurrent(latestSize)
	}

	if ctx != nil && ctx.Err() != nil {
		t.Cancel("")
		return
	}
}

func (p *repoDownloadProgress) Retry(rawURL string, attempt, maxAttempts int, err error) {
	if p == nil {
		return
	}
	t := p.taskForURL(rawURL)
	if t == nil {
		return
	}
	t.Retrying(fmt.Sprintf("retrying %d/%d...", attempt, maxAttempts))
}

func (p *repoDownloadProgress) Success(rawURL string) {
	if p == nil {
		return
	}
	t := p.taskForURL(rawURL)
	if t == nil {
		return
	}
	t.SetMessage("")
	t.Done()
}

func (p *repoDownloadProgress) Error(rawURL string, attempt, maxAttempts int, err error) {
	if p == nil {
		return
	}
	t := p.taskForURL(rawURL)
	if t == nil {
		return
	}
	if err == nil {
		t.Error("download failed")
		return
	}
	t.Error(err.Error())
}

func (p *repoDownloadProgress) taskForURL(rawURL string) *progressv2.Task {
	if p == nil {
		return nil
	}

	base := downloadTitle(rawURL)
	componentID, resolved, ok := parseComponentVersionFromTarball(base)
	if ok {
		key := componentID + "@" + resolved
		p.mu.Lock()
		t := p.expected[key]
		p.mu.Unlock()
		if t != nil {
			return t
		}
	}

	p.mu.Lock()
	t := p.byURL[rawURL]
	p.mu.Unlock()
	return t
}

func downloadDisplay(rawURL string) (name, version string) {
	base := downloadTitle(rawURL)
	component, v, ok := parseComponentVersionFromTarball(base)
	if !ok {
		// Fallback to the raw filename to avoid hiding information for unknown
		// patterns.
		return base, ""
	}
	return proc.ComponentDisplayName(proc.RepoComponentID(component)), v
}

func downloadTitle(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil && u.Path != "" {
		if base := path.Base(u.Path); base != "" && base != "." && base != "/" {
			return base
		}
	}
	// Fallback: best-effort base name on the original string.
	if base := path.Base(rawURL); base != "" && base != "." && base != "/" {
		return base
	}
	return rawURL
}

func parseComponentVersionFromTarball(filename string) (component, version string, ok bool) {
	name := strings.TrimSuffix(filename, ".tar.gz")
	if name == filename {
		// Only attempt parsing on the known tiup tarball naming convention.
		return "", "", false
	}

	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return "", "", false
	}

	// Drop the trailing platform suffix ("<goos>-<goarch>") when present.
	if len(parts) >= 4 && isKnownGOOS(parts[len(parts)-2]) && isKnownGOARCH(parts[len(parts)-1]) {
		parts = parts[:len(parts)-2]
	}

	versionStart := -1
	for i := 1; i < len(parts); i++ {
		if looksLikeVersionPart(parts[i]) {
			versionStart = i
			break
		}
	}
	if versionStart <= 0 {
		return "", "", false
	}

	component = strings.Join(parts[:versionStart], "-")
	version = strings.Join(parts[versionStart:], "-")
	return component, version, true
}

func looksLikeVersionPart(part string) bool {
	if part == "" {
		return false
	}
	switch part {
	case "nightly", "latest":
		return true
	}
	// Common semver form is either "v1.2.3" or "1.2.3".
	if len(part) >= 2 && part[0] == 'v' && part[1] >= '0' && part[1] <= '9' {
		return true
	}
	if part[0] >= '0' && part[0] <= '9' {
		return true
	}
	return false
}

func isKnownGOOS(goos string) bool {
	switch goos {
	case "linux", "darwin", "windows":
		return true
	default:
		return false
	}
}

func isKnownGOARCH(goarch string) bool {
	switch goarch {
	case "amd64", "arm64", "arm", "386", "ppc64le", "s390x", "riscv64":
		return true
	default:
		return false
	}
}

var _ repository.DownloadProgress = (*repoDownloadProgress)(nil)
var _ repository.DownloadProgressReporter = (*repoDownloadProgress)(nil)

func main() {
	tui.RegisterArg0(playgroundCLIArg0())

	state := newCLIState()

	code := 0
	err := execute(state)
	if err != nil {
		var rendered renderedError
		if !stdErrors.As(err, &rendered) {
			out := tuiv2output.Stderr.Get()
			colorstr.Fprintf(out, "[red][bold]Error:[reset] %v\n", err)
		}
		code = 1
	}
	if state != nil && state.destroyDataAfterExit && state.tiupDataDir == "" && state.dataDir != "" {
		_ = os.RemoveAll(state.dataDir)
	}

	if code != 0 {
		os.Exit(code)
	}
}
