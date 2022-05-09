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

package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/manager"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	tiupmeta "github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/logger"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	errNS         = errorx.NewNamespace("cmd")
	rootCmd       *cobra.Command
	gOpt          operator.Options
	skipConfirm   bool
	reportEnabled bool // is telemetry report enabled
	teleReport    *telemetry.Report
	clusterReport *telemetry.ClusterReport
	teleNodeInfos []*telemetry.NodeInfo
	teleTopology  string
	teleCommand   []string
	log           = logprinter.NewLogger("") // init default logger
)

var tidbSpec *spec.SpecManager
var cm *manager.Manager

func scrubClusterName(n string) string {
	// prepend the telemetry secret to cluster name, so that two installations
	// of tiup with the same cluster name produce different hashes
	return "cluster_" + telemetry.SaltedHash(n)
}

func getParentNames(cmd *cobra.Command) []string {
	if cmd == nil {
		return nil
	}

	p := cmd.Parent()
	// always use 'cluster' as the root command name
	if cmd.Parent() == nil {
		return []string{"cluster"}
	}

	return append(getParentNames(p), cmd.Name())
}

func init() {
	logger.InitGlobalLogger()

	tui.AddColorFunctionsForCobra()

	cobra.EnableCommandSorting = false

	nativeEnvVar := strings.ToLower(os.Getenv(localdata.EnvNameNativeSSHClient))
	if nativeEnvVar == "true" || nativeEnvVar == "1" || nativeEnvVar == "enable" {
		gOpt.NativeSSH = true
	}

	rootCmd = &cobra.Command{
		Use:           tui.OsArgs0(),
		Short:         "Deploy a TiDB cluster for production",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiUPVersion().String(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// populate logger
			log.SetDisplayModeFromString(gOpt.DisplayMode)

			var err error
			var env *tiupmeta.Environment
			if err = spec.Initialize("cluster"); err != nil {
				return err
			}

			tidbSpec = spec.GetSpecManager()
			cm = manager.NewManager("tidb", tidbSpec, spec.TiDBComponentVersion, log)
			if cmd.Name() != "__complete" {
				logger.EnableAuditLog(spec.AuditDir())
			}

			// Running in other OS/ARCH Should be fine we only download manifest file.
			env, err = tiupmeta.InitEnv(repository.Options{
				GOOS:   "linux",
				GOARCH: "amd64",
			}, repository.MirrorOptions{})
			if err != nil {
				return err
			}
			tiupmeta.SetGlobalEnv(env)

			teleCommand = getParentNames(cmd)

			if gOpt.NativeSSH {
				gOpt.SSHType = executor.SSHTypeSystem
				log.Infof(
					"System ssh client will be used (%s=%s)",
					localdata.EnvNameNativeSSHClient,
					os.Getenv(localdata.EnvNameNativeSSHClient))
				log.Infof("The --native-ssh flag has been deprecated, please use --ssh=system")
			}

			err = proxy.MaybeStartProxy(
				gOpt.SSHProxyHost,
				gOpt.SSHProxyPort,
				gOpt.SSHProxyUser,
				gOpt.SSHProxyUsePassword,
				gOpt.SSHProxyIdentity,
				log,
			)
			if err != nil {
				return perrs.Annotate(err, "start http-proxy")
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			proxy.MaybeStopProxy()
			return tiupmeta.GlobalEnv().V1Repository().Mirror().Close()
		},
	}

	tui.BeautifyCobraUsageAndHelp(rootCmd)

	rootCmd.PersistentFlags().Uint64Var(&gOpt.SSHTimeout, "ssh-timeout", 5, "Timeout in seconds to connect host via SSH, ignored for operations that don't need an SSH connection.")
	// the value of wait-timeout is also used for `systemctl` commands, as the default timeout of systemd for
	// start/stop operations is 90s, the default value of this argument is better be longer than that
	rootCmd.PersistentFlags().Uint64Var(&gOpt.OptTimeout, "wait-timeout", 120, "Timeout in seconds to wait for an operation to complete, ignored for operations that don't fit.")
	rootCmd.PersistentFlags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip all confirmations and assumes 'yes'")
	rootCmd.PersistentFlags().BoolVar(&gOpt.NativeSSH, "native-ssh", gOpt.NativeSSH, "(EXPERIMENTAL) Use the native SSH client installed on local system instead of the build-in one.")
	rootCmd.PersistentFlags().StringVar((*string)(&gOpt.SSHType), "ssh", "", "(EXPERIMENTAL) The executor type: 'builtin', 'system', 'none'.")
	rootCmd.PersistentFlags().IntVarP(&gOpt.Concurrency, "concurrency", "c", 5, "max number of parallel tasks allowed")
	rootCmd.PersistentFlags().StringVar(&gOpt.DisplayMode, "format", "default", "(EXPERIMENTAL) The format of output, available values are [default, json]")
	rootCmd.PersistentFlags().StringVar(&gOpt.SSHProxyHost, "ssh-proxy-host", "", "The SSH proxy host used to connect to remote host.")
	rootCmd.PersistentFlags().StringVar(&gOpt.SSHProxyUser, "ssh-proxy-user", utils.CurrentUser(), "The user name used to login the proxy host.")
	rootCmd.PersistentFlags().IntVar(&gOpt.SSHProxyPort, "ssh-proxy-port", 22, "The port used to login the proxy host.")
	rootCmd.PersistentFlags().StringVar(&gOpt.SSHProxyIdentity, "ssh-proxy-identity-file", path.Join(utils.UserHome(), ".ssh", "id_rsa"), "The identity file used to login the proxy host.")
	rootCmd.PersistentFlags().BoolVar(&gOpt.SSHProxyUsePassword, "ssh-proxy-use-password", false, "Use password to login the proxy host.")
	rootCmd.PersistentFlags().Uint64Var(&gOpt.SSHProxyTimeout, "ssh-proxy-timeout", 5, "Timeout in seconds to connect the proxy host via SSH, ignored for operations that don't need an SSH connection.")
	_ = rootCmd.PersistentFlags().MarkHidden("native-ssh")
	_ = rootCmd.PersistentFlags().MarkHidden("ssh-proxy-host")
	_ = rootCmd.PersistentFlags().MarkHidden("ssh-proxy-user")
	_ = rootCmd.PersistentFlags().MarkHidden("ssh-proxy-port")
	_ = rootCmd.PersistentFlags().MarkHidden("ssh-proxy-identity-file")
	_ = rootCmd.PersistentFlags().MarkHidden("ssh-proxy-use-password")
	_ = rootCmd.PersistentFlags().MarkHidden("ssh-proxy-timeout")

	rootCmd.AddCommand(
		newCheckCmd(),
		newDeploy(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newScaleInCmd(),
		newScaleOutCmd(),
		newDestroyCmd(),
		newCleanCmd(),
		newUpgradeCmd(),
		newDisplayCmd(),
		newPruneCmd(),
		newListCmd(),
		newAuditCmd(),
		newImportCmd(),
		newEditConfigCmd(),
		newShowConfigCmd(),
		newReloadCmd(),
		newPatchCmd(),
		newRenameCmd(),
		newEnableCmd(),
		newDisableCmd(),
		newExecCmd(),
		newPullCmd(),
		newPushCmd(),
		newTestCmd(), // hidden command for test internally
		newTelemetryCmd(),
		newReplayCmd(),
		newTemplateCmd(),
		newTLSCmd(),
		newMetaCmd(),
	)
}

func printErrorMessageForNormalError(err error) {
	_, _ = tui.ColorErrorMsg.Fprintf(os.Stderr, "\nError: %s\n", err.Error())
}

func printErrorMessageForErrorX(err *errorx.Error) {
	msg := ""
	ident := 0
	causeErrX := err
	for causeErrX != nil {
		if ident > 0 {
			msg += strings.Repeat("  ", ident) + "caused by: "
		}
		currentErrMsg := causeErrX.Message()
		if len(currentErrMsg) > 0 {
			if ident == 0 {
				// Print error code only for top level error
				msg += fmt.Sprintf("%s (%s)\n", currentErrMsg, causeErrX.Type().FullName())
			} else {
				msg += fmt.Sprintf("%s\n", currentErrMsg)
			}
			ident++
		}
		cause := causeErrX.Cause()
		if c := errorx.Cast(cause); c != nil {
			causeErrX = c
		} else {
			if cause != nil {
				if ident > 0 {
					// The error may have empty message. In this case we treat it as a transparent error.
					// Thus `ident == 0` can be possible.
					msg += strings.Repeat("  ", ident) + "caused by: "
				}
				msg += fmt.Sprintf("%s\n", cause.Error())
			}
			break
		}
	}
	_, _ = tui.ColorErrorMsg.Fprintf(os.Stderr, "\nError: %s", msg)
}

func extractSuggestionFromErrorX(err *errorx.Error) string {
	cause := err
	for cause != nil {
		v, ok := cause.Property(utils.ErrPropSuggestion)
		if ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		cause = errorx.Cast(cause.Cause())
	}

	return ""
}

// Execute executes the root command
func Execute() {
	zap.L().Info("Execute command", zap.String("command", tui.OsArgs()))
	zap.L().Debug("Environment variables", zap.Strings("env", os.Environ()))

	teleReport = new(telemetry.Report)
	clusterReport = new(telemetry.ClusterReport)
	teleReport.EventDetail = &telemetry.Report_Cluster{Cluster: clusterReport}
	reportEnabled = telemetry.Enabled()
	if reportEnabled {
		eventUUID := os.Getenv(localdata.EnvNameTelemetryEventUUID)
		if eventUUID == "" {
			eventUUID = uuid.New().String()
		}
		teleReport.InstallationUUID = telemetry.GetUUID()
		teleReport.EventUUID = eventUUID
		teleReport.EventUnixTimestamp = time.Now().Unix()
		teleReport.Version = telemetry.TiUPMeta()
	}

	start := time.Now()
	code := 0
	err := rootCmd.Execute()
	if err != nil {
		code = 1
	}

	zap.L().Info("Execute command finished", zap.Int("code", code), zap.Error(err))

	if reportEnabled {
		f := func() {
			defer func() {
				if r := recover(); r != nil {
					if environment.DebugMode {
						log.Debugf("Recovered in telemetry report: %v", r)
					}
				}
			}()

			clusterReport.ExitCode = int32(code)
			clusterReport.Nodes = teleNodeInfos
			if teleTopology != "" {
				if data, err := telemetry.ScrubYaml(
					[]byte(teleTopology),
					map[string]struct{}{
						"host":       {},
						"name":       {},
						"user":       {},
						"group":      {},
						"deploy_dir": {},
						"data_dir":   {},
						"log_dir":    {},
					}, // fields to hash
					map[string]struct{}{
						"config":         {},
						"server_configs": {},
					}, // fields to omit
					telemetry.GetSecret(),
				); err == nil {
					clusterReport.Topology = (string(data))
				}
			}
			clusterReport.TakeMilliseconds = uint64(time.Since(start).Milliseconds())
			clusterReport.Command = strings.Join(teleCommand, " ")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			tele := telemetry.NewTelemetry()
			err := tele.Report(ctx, teleReport)
			if environment.DebugMode {
				if err != nil {
					log.Infof("report failed: %v", err)
				}
				log.Errorf("report: %s\n", teleReport.String())
				if data, err := json.Marshal(teleReport); err == nil {
					log.Debugf("report: %s\n", string(data))
				}
			}
			cancel()
		}

		f()
	}

	switch log.GetDisplayMode() {
	case logprinter.DisplayModeJSON:
		obj := struct {
			Code int    `json:"exit_code"`
			Err  string `json:"error,omitempty"`
		}{
			Code: code,
		}
		if err != nil {
			obj.Err = err.Error()
		}
		data, err := json.Marshal(obj)
		if err != nil {
			fmt.Printf("{\"exit_code\":%d, \"error\":\"%s\"}", code, err)
		}
		fmt.Fprintln(os.Stderr, string(data))
	default:
		if err != nil {
			if errx := errorx.Cast(err); errx != nil {
				printErrorMessageForErrorX(errx)
			} else {
				printErrorMessageForNormalError(err)
			}

			if !errorx.HasTrait(err, utils.ErrTraitPreCheck) {
				logger.OutputDebugLog("tiup-cluster")
			}

			if errx := errorx.Cast(err); errx != nil {
				if suggestion := extractSuggestionFromErrorX(errx); len(suggestion) > 0 {
					log.Errorf("\n%s\n", suggestion)
				}
			}
		}
	}
	err = logger.OutputAuditLogIfEnabled()
	if err != nil {
		zap.L().Warn("Write audit log file failed", zap.Error(err))
		code = 1
	}

	color.Unset()

	if code != 0 {
		os.Exit(code)
	}
}
