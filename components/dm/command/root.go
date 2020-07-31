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
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap/tiup/components/dm/spec"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster"
	"github.com/pingcap/tiup/pkg/cluster/flags"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/colorutil"
	tiupmeta "github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/errutil"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	// nolint
	errNS       = errorx.NewNamespace("cmd")
	rootCmd     *cobra.Command
	gOpt        operator.Options
	skipConfirm bool
)

var dmspec *cspec.SpecManager
var manager *cluster.Manager

func init() {
	logger.InitGlobalLogger()

	colorutil.AddColorFunctionsForCobra()

	// Initialize the global variables
	flags.ShowBacktrace = len(os.Getenv("TIUP_BACKTRACE")) > 0
	cobra.EnableCommandSorting = false

	nativeEnvVar := strings.ToLower(os.Getenv(localdata.EnvNameNativeSSHClient))
	if nativeEnvVar == "true" || nativeEnvVar == "1" || nativeEnvVar == "enable" {
		gOpt.NativeSSH = true
	}

	rootCmd = &cobra.Command{
		Use:           cliutil.OsArgs0(),
		Short:         "Deploy a DM cluster for production",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiUPVersion().String(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			var env *tiupmeta.Environment
			if err = cspec.Initialize("dm"); err != nil {
				return err
			}

			dmspec = spec.GetSpecManager()
			logger.EnableAuditLog(cspec.AuditDir())
			manager = cluster.NewManager("dm", spec.GetSpecManager())

			// Running in other OS/ARCH Should be fine we only download manifest file.
			env, err = tiupmeta.InitEnv(repository.Options{
				GOOS:   "linux",
				GOARCH: "amd64",
			})
			if err != nil {
				return err
			}
			tiupmeta.SetGlobalEnv(env)

			if gOpt.NativeSSH {
				zap.L().Info("Native ssh client will be used",
					zap.String(localdata.EnvNameNativeSSHClient, os.Getenv(localdata.EnvNameNativeSSHClient)))
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return tiupmeta.GlobalEnv().V1Repository().Mirror().Close()
		},
	}

	cliutil.BeautifyCobraUsageAndHelp(rootCmd)

	rootCmd.PersistentFlags().Int64Var(&gOpt.SSHTimeout, "ssh-timeout", 5, "Timeout in seconds to connect host via SSH, ignored for operations that don't need an SSH connection.")
	rootCmd.PersistentFlags().Int64Var(&gOpt.OptTimeout, "wait-timeout", 60, "Timeout in seconds to wait for an operation to complete, ignored for operations that don't fit.")
	rootCmd.PersistentFlags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip all confirmations and assumes 'yes'")
	rootCmd.PersistentFlags().BoolVar(&gOpt.NativeSSH, "native-ssh", gOpt.NativeSSH, "Use the native SSH client installed on local system instead of the build-in one.")

	rootCmd.AddCommand(
		newDeploy(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newListCmd(),
		newDestroyCmd(),
		newAuditCmd(),
		newExecCmd(),
		newEditConfigCmd(),
		newDisplayCmd(),
		newReloadCmd(),
		newUpgradeCmd(),
		newPatchCmd(),
		newScaleOutCmd(),
		newScaleInCmd(),
	)
}

func printErrorMessageForNormalError(err error) {
	_, _ = colorutil.ColorErrorMsg.Fprintf(os.Stderr, "\nError: %s\n", err.Error())
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
		} else if cause != nil {
			if ident > 0 {
				// Out most error may have empty message. In this case we treat it as a transparent error.
				// Thus `ident == 0` can be possible.
				msg += strings.Repeat("  ", ident) + "caused by: "
			}
			msg += fmt.Sprintf("%s\n", cause.Error())
			break
		} else {
			break
		}
	}
	_, _ = colorutil.ColorErrorMsg.Fprintf(os.Stderr, "\nError: %s", msg)
}

func extractSuggestionFromErrorX(err *errorx.Error) string {
	cause := err
	for cause != nil {
		v, ok := cause.Property(errutil.ErrPropSuggestion)
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
	zap.L().Info("Execute command", zap.String("command", cliutil.OsArgs()))
	zap.L().Debug("Environment variables", zap.Strings("env", os.Environ()))

	// Switch current work directory if running in TiUP component mode
	if wd := os.Getenv(localdata.EnvNameWorkDir); wd != "" {
		if err := os.Chdir(wd); err != nil {
			zap.L().Warn("Failed to switch work directory", zap.String("working_dir", wd), zap.Error(err))
		}
	}

	code := 0
	err := rootCmd.Execute()
	if err != nil {
		code = 1
	}

	zap.L().Info("Execute command finished", zap.Int("code", code), zap.Error(err))

	if err != nil {
		if errx := errorx.Cast(err); errx != nil {
			printErrorMessageForErrorX(errx)
		} else {
			printErrorMessageForNormalError(err)
		}

		if !errorx.HasTrait(err, errutil.ErrTraitPreCheck) {
			logger.OutputDebugLog()
		}

		if errx := errorx.Cast(err); errx != nil {
			if suggestion := extractSuggestionFromErrorX(errx); len(suggestion) > 0 {
				_, _ = fmt.Fprintf(os.Stderr, "\n%s\n", suggestion)
			}
		}
	}

	logger.OutputAuditLogIfEnabled()

	color.Unset()

	if code != 0 {
		os.Exit(code)
	}
}
