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
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
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
	options        = &BootOptions{}
	tag            string
	deleteWhenExit bool
	tiupDataDir    string
	dataDir        string
	log            = logprinter.NewLogger("")
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

	w := ui.Writer()

	// Keep all user-facing output consistent with the resolved output mode.
	color.NoColor = !tuiterm.Resolve(w).Color

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
	}
}

func printInterrupt(ui *progressv2.UI, sig syscall.Signal) {
	if ui == nil {
		return
	}

	msg := fmt.Sprintf("Playground receive signal: %s", sig)

	prefix := ""
	if sig == syscall.SIGINT {
		prefix = "^C"
	}
	colorstr.Fprintf(ui.Writer(), "%s[red][bold]%s[reset]\n", prefix, msg)
}

func execute() error {
	rootCmd := &cobra.Command{
		Use: fmt.Sprintf("%s [version]", filepath.Base(os.Args[0])),
		Long: `Bootstrap a TiDB cluster in your local host, the latest release version will be chosen
if you don't specified a version.

Examples:
  $ tiup playground nightly                         # Start a TiDB nightly version local cluster
  $ tiup playground v5.0.1 --db 3 --pd 3 --kv 3     # Start a local cluster with 10 nodes
  $ tiup playground nightly --without-monitor       # Start a local cluster and disable monitor system
  $ tiup playground --pd.config ~/config/pd.toml    # Start a local cluster with specified configuration file
  $ tiup playground --db.binpath /xx/tidb-server    # Start a local cluster with component binary path
  $ tiup playground --tag xx                           # Start a local cluster with data dir named 'xx' and uncleaned after exit
  $ tiup playground --mode tikv-slim                # Start a local tikv only cluster (No TiDB or TiFlash Available)
  $ tiup playground --mode tikv-slim --kv 3 --pd 3  # Start a local tikv only cluster with 6 nodes`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiUPVersion().String(),
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			tiupDataDir = os.Getenv(localdata.EnvNameInstanceDataDir)
			tiupHome := os.Getenv(localdata.EnvNameHome)
			if tiupHome == "" {
				tiupHome, _ = getAbsolutePath(filepath.Join("~", localdata.ProfileDirName))
			}

			isRoot := cmd.Parent() == nil

			switch {
			case tag != "":
				dataDir = filepath.Join(tiupHome, localdata.DataParentDir, tag)
			case tiupDataDir != "":
				dataDir = tiupDataDir
				tag = filepath.Base(dataDir)
			default:
				if isRoot {
					tag = utils.Base62Tag()
					dataDir = filepath.Join(tiupHome, localdata.DataParentDir, tag)
					deleteWhenExit = true
				} else {
					dataDir = filepath.Join(tiupHome, localdata.DataParentDir)
				}
			}

			if isRoot {
				err := utils.MkdirAll(dataDir, os.ModePerm)
				if err != nil {
					return err
				}
				if out := tuiv2output.Stdout.Get(); tuiterm.Resolve(out).Control {
					_, _ = fmt.Fprintf(out, "\033]0;TiUP Playground: %s\a", tag)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				options.Version = args[0]
			} else if options.ShOpt.Mode == proc.ModeNextGen {
				options.Version = fmt.Sprintf("%s-%s", utils.LatestVersionAlias, utils.NextgenVersionAlias)
			}

			if err := populateDefaultOpt(cmd.Flags()); err != nil {
				return err
			}

			port := utils.MustGetFreePort("0.0.0.0", 9527, options.ShOpt.PortOffset)
			err := dumpPort(filepath.Join(dataDir, "port"), port)
			p := NewPlayground(dataDir, port)
			if err != nil {
				return err
			}

			ui := progressv2.New(progressv2.Options{Mode: progressv2.ModeAuto, Out: os.Stderr})
			defer ui.Close()
			p.ui = ui
			p.downloadGroup = ui.Group("Downloading components")
			p.downloadGroup.SetHideDetailsOnSuccess(true)
			p.downloadGroup.SetSortTasksByTitle(true)
			p.startingGroup = ui.Group("Starting instances")
			downloadGroup := p.downloadGroup
			restore := attachUIOutput(ui)
			defer restore()

			env, err := environment.InitEnv(repository.Options{}, repository.MirrorOptions{
				Progress: newRepoDownloadProgress(downloadGroup),
			})
			if err != nil {
				return err
			}
			environment.SetGlobalEnv(env)

			var (
				booted      uint32
				sigReceived uint32
			)
			ctx, cancel := context.WithCancelCause(context.Background())
			ctx = context.WithValue(ctx, logprinter.ContextKeyLogger, log)
			defer cancel(nil)
			p.bootCancel = cancel

			go func() {
				sc := make(chan os.Signal, 1)
				signal.Notify(sc,
					syscall.SIGHUP,
					syscall.SIGINT,
					syscall.SIGTERM,
					syscall.SIGQUIT,
				)

				sig := (<-sc).(syscall.Signal)
				atomic.StoreUint32(&sigReceived, 1)

				// if bootCluster is not done we just cancel context to make it
				// clean up and return ASAP and exit directly after timeout.
				// Note now bootCluster can not learn the context is done and return quickly now
				// like while it's downloading component.
				if atomic.LoadUint32(&booted) == 0 {
					cancel(nil)
				}
				p.requestStopSignal(sig)
				// If user try double ctrl+c, force quit
				sig2 := (<-sc).(syscall.Signal)
				if sig2 == syscall.SIGINT {
					p.requestForceKill()
				}
			}()

			bootErr := p.bootCluster(ctx, options)
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
					out := p.termWriter()

					fmt.Fprintln(out)
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
			return environment.GlobalEnv().Close()
		},
	}

	// Cobra's help template uses color escape sequences computed at registration
	// time. Make sure it follows the same progress mode decision as the rest of
	// the playground output.
	if !tuiterm.ResolveFile(os.Stdout).Color || !tuiterm.ResolveFile(os.Stderr).Color {
		color.NoColor = true
	}

	tui.AddColorFunctionsForCobra()
	tui.BeautifyCobraUsageAndHelp(rootCmd)

	rootCmd.Flags().StringVar(&options.ShOpt.Mode, "mode", "tidb", fmt.Sprintf("TiUP playground mode: '%s', '%s', '%s', '%s', '%s'", proc.ModeNormal, proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg, proc.ModeTiKVSlim))
	rootCmd.Flags().StringVar(&options.ShOpt.PDMode, "pd.mode", "pd", "PD mode: 'pd', 'ms'")
	rootCmd.Flags().StringVar(&options.ShOpt.CSE.S3Endpoint, "cse.s3_endpoint", "http://127.0.0.1:9000",
		fmt.Sprintf("Object store URL for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().StringVar(&options.ShOpt.CSE.Bucket, "cse.bucket", "tiflash",
		fmt.Sprintf("Object store bucket for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().StringVar(&options.ShOpt.CSE.AccessKey, "cse.access_key", "minioadmin",
		fmt.Sprintf("Object store access key for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().StringVar(&options.ShOpt.CSE.SecretKey, "cse.secret_key", "minioadmin",
		fmt.Sprintf("Object store secret key for --mode=%s, --mode=%s, --mode=%s", proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen))
	rootCmd.Flags().BoolVar(&options.ShOpt.HighPerf, "perf", false, "Tune default config for better performance instead of debug troubleshooting")
	rootCmd.Flags().BoolVar(&options.ShOpt.EnableTiKVColumnar, "tikv.columnar", false,
		fmt.Sprintf("Enable TiKV columnar storage engine, only available when --mode=%s", proc.ModeCSE))
	rootCmd.Flags().BoolVar(&options.ShOpt.ForcePull, "force-pull", false, "Force redownload the component. It is useful to manually refresh nightly or broken binaries")

	rootCmd.PersistentFlags().StringVarP(&tag, "tag", "T", "", "Specify a tag for playground, data dir of this tag will not be removed after exit")
	rootCmd.Flags().Bool("without-monitor", false, "Don't start prometheus and grafana component")
	rootCmd.Flags().BoolVar(&options.Monitor, "monitor", true, "Start prometheus and grafana component")
	_ = rootCmd.Flags().MarkDeprecated("monitor", "Please use --without-monitor to control whether to disable monitor.")
	rootCmd.Flags().IntVar(&options.GrafanaPort, "grafana.port", 3000, "grafana port. If not provided, grafana will use 3000 as its port.")
	rootCmd.Flags().IntVar(&options.ShOpt.PortOffset, "port-offset", 0, "If specified, all components will use default_port+port_offset as the port. This argument is useful when you want to start multiple playgrounds on the same host. Recommend to set to 10000, 20000, etc.")

	// NOTE: Do not set default values if they may be changed in different modes.

	registerServiceFlags(rootCmd.Flags(), options)

	rootCmd.Flags().StringVar(&options.Host, "host", "127.0.0.1", "Playground cluster host")

	rootCmd.AddCommand(newDisplay())
	rootCmd.AddCommand(newScaleOut())
	rootCmd.AddCommand(newScaleIn())

	return rootCmd.Execute()
}

func populateDefaultOpt(flagSet *pflag.FlagSet) error {
	if flagSet.Lookup("without-monitor").Changed {
		v, _ := flagSet.GetBool("without-monitor")
		options.Monitor = !v
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

	port, err = strconv.Atoi(string(data))
	return
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

func main() {
	tui.RegisterArg0("tiup playground")

	code := 0
	err := execute()
	if err != nil {
		var rendered renderedError
		if !stdErrors.As(err, &rendered) {
			out := tuiv2output.Stderr.Get()
			colorstr.Fprintf(out, "[red][bold]Error:[reset] %v\n", err)
		}
		code = 1
	}
	removeData()

	if code != 0 {
		os.Exit(code)
	}
}

func removeData() {
	if deleteWhenExit {
		os.RemoveAll(dataDir)
	}
}
