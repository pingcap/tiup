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

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/client"
	"github.com/pingcap/tiup/pkg/environment"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/localdata"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
)

var (
	rootCmd       *cobra.Command
	repoOpts      repository.Options
	reportEnabled bool // is telemetry report enabled
	eventUUID     = uuid.New().String()
	teleCommand   string
	log           = logprinter.NewLogger("") // use default logger
)

// arguments
var (
	binPath string
	tag     string
	tiupC   *client.Client
)

func init() {
	cobra.EnableCommandSorting = false
	_ = os.Setenv(localdata.EnvNameTelemetryEventUUID, eventUUID)

	rootCmd = &cobra.Command{
		Use: `tiup [flags] <command> [args...]
  tiup [flags] <component> [args...]`,
		Long: `TiUP is a command-line component management tool that can help to download and install
TiDB platform components to the local system. You can run a specific version of a component via
"tiup <component>[:version]". If no version number is specified, the latest version installed
locally will be used. If the specified component does not have any version installed locally,
the latest stable version will be downloaded from the repository.`,
		Example: `  $ tiup playground                    # Quick start
  $ tiup playground nightly            # Start a playground with the latest nightly version
  $ tiup install <component>[:version] # Install a component of specific version
  $ tiup update --all                  # Update all installed components to the latest version
  $ tiup update --nightly              # Update all installed components to the nightly version
  $ tiup update --self                 # Update the "tiup" to the latest version
  $ tiup list                          # Fetch the latest supported components list
  $ tiup status                        # Display all running/terminated instances
  $ tiup clean <name>                  # Clean the data of running/terminated instance (Kill process if it's running)
  $ tiup clean --all                   # Clean the data of all running/terminated instances`,

		SilenceErrors:      true,
		DisableFlagParsing: true,
		Args: func(cmd *cobra.Command, args []string) error {
			// Support `tiup <component>`
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			switch cmd.Name() {
			case "init",
				"rotate",
				"set":
				if cmd.HasParent() && cmd.Parent().Name() == "mirror" {
					// skip environment init
					break
				}
				fallthrough
			default:
				e, err := environment.InitEnv(repoOpts, repository.MirrorOptions{})
				if err != nil {
					if errors.Is(perrs.Cause(err), v1manifest.ErrLoadManifest) {
						log.Warnf("Please check for root manifest file, you may download one from the repository mirror, or try `tiup mirror set` to force reset it.")
					}
					return err
				}
				environment.SetGlobalEnv(e)

				tiupC, err = client.NewTiUPClient(os.Getenv(localdata.EnvNameHome))
				if err != nil {
					return err
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			env := environment.GlobalEnv()

			// TBD: change this flag to subcommand

			// We assume the first unknown parameter is the component name and following
			// parameters will be transparent passed because registered flags and subcommands
			// will be parsed correctly.
			// e.g: tiup --tag mytag --rm playground --db 3 --pd 3 --kv 4
			//   => run "playground" with parameters "--db 3 --pd 3 --kv 4"
			// tiup --tag mytag --binpath /xxx/tikv-server tikv
			switch args[0] {
			case "--help", "-h":
				return cmd.Help()
			case "--version", "-v":
				fmt.Println(version.NewTiUPVersion().String())
				return nil
			case "--binary":
				if len(args) < 2 {
					return fmt.Errorf("flag needs an argument: %s", args[0])
				}
				component, ver := environment.ParseCompVersion(args[1])
				selectedVer, err := env.SelectInstalledVersion(component, ver)
				if err != nil {
					return err
				}
				binaryPath, err := env.BinaryPath(component, selectedVer)
				if err != nil {
					return err
				}
				fmt.Println(binaryPath)
				return nil
			case "--binpath":
				if len(args) < 2 {
					return fmt.Errorf("flag %s needs an argument", args[0])
				}
				binPath = args[1]
				args = args[2:]
			case "--tag", "-T":
				if len(args) < 2 {
					return fmt.Errorf("flag %s needs an argument", args[0])
				}
				tag = args[1]
				args = args[2:]
			}

			// component may use tag from environment variable. as workaround, make tiup set the same tag
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--tag" || args[i] == "-T" {
					tag = args[i+1]
				}
			}

			if len(args) < 1 {
				return cmd.Help()
			}

			componentSpec := args[0]
			args = args[1:]
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}

			teleCommand = fmt.Sprintf("%s %s", cmd.CommandPath(), componentSpec)
			return tiupexec.RunComponent(tiupC, env, tag, componentSpec, binPath, args)
		},
		SilenceUsage: true,
		// implement auto completion for tiup components
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			env := environment.GlobalEnv()
			if len(args) == 0 {
				var result []string
				installed, _ := env.Profile().InstalledComponents()
				for _, comp := range installed {
					if strings.HasPrefix(comp, toComplete) {
						result = append(result, comp)
					}
				}
				return result, cobra.ShellCompDirectiveNoFileComp
			}

			component, version := environment.ParseCompVersion(args[0])

			selectedVer, err := env.SelectInstalledVersion(component, version)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			binaryPath, err := env.BinaryPath(component, selectedVer)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			argv := []string{binaryPath, "__complete"}
			argv = append(append(argv, args[1:]...), toComplete)
			_ = syscall.Exec(binaryPath, argv, os.Environ())
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	// useless, exist to generate help information
	rootCmd.Flags().String("binary", "", "Print binary path of a specific version of a component `<component>[:version]`\n"+
		"and the latest version installed will be selected if no version specified")
	rootCmd.Flags().StringP("tag", "T", "", "[Deprecated] Specify a tag for component instance")
	rootCmd.Flags().String("binpath", "", "Specify the binary path of component instance")
	rootCmd.Flags().BoolP("version", "v", false, "Print the version of tiup")

	rootCmd.AddCommand(
		newInstallCmd(),
		newListCmd(),
		newUninstallCmd(),
		newUpdateCmd(),
		newStatusCmd(),
		newCleanCmd(),
		newMirrorCmd(),
		newTelemetryCmd(),
		newEnvCmd(),
		newHistoryCmd(),
	)
}

// Execute parses the command line arguments and calls proper functions
func Execute() {
	start := time.Now()
	code := 0

	err := rootCmd.Execute()
	if err != nil {
		// use exit code from component
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			fmt.Fprintln(os.Stderr, color.RedString("Error: %v", err))
			code = 1
		}
	}

	teleReport := new(telemetry.Report)
	tiupReport := new(telemetry.TiUPReport)
	teleReport.EventDetail = &telemetry.Report_Tiup{Tiup: tiupReport}

	env := environment.GlobalEnv()
	if env == nil {
		// if the env is not initialized, skip telemetry upload
		// as many info are read from the env.
		// TODO: split pure meta information from env object and
		// us a dedicated package for that
		reportEnabled = false
	} else {
		// record TiUP execution history
		err := environment.HistoryRecord(env, os.Args, start, code)
		if err != nil {
			log.Warnf("Record TiUP execution history log failed: %v", err)
		}

		teleMeta, _, err := telemetry.GetMeta(env)
		if err == nil {
			reportEnabled = teleMeta.Status == telemetry.EnableStatus
			teleReport.InstallationUUID = teleMeta.UUID
		} // default to false on errors
	}
	if teleCommand == "tiup __complete" {
		reportEnabled = false
	}

	if reportEnabled {
		teleReport.EventUUID = eventUUID
		teleReport.EventUnixTimestamp = start.Unix()
		teleReport.Version = telemetry.TiUPMeta()
		teleReport.Version.TiUPVersion = version.NewTiUPVersion().SemVer()
		tiupReport.Command = teleCommand
		tiupReport.CustomMirror = env.Profile().Config.Mirror != repository.DefaultMirror
		if tag != "" {
			tiupReport.Tag = telemetry.SaltedHash(tag)
		}

		f := func() {
			defer func() {
				if r := recover(); r != nil {
					if environment.DebugMode {
						log.Debugf("Recovered in telemetry report: %v", r)
					}
				}
			}()

			tiupReport.ExitCode = int32(code)
			tiupReport.TakeMilliseconds = uint64(time.Since(start).Milliseconds())
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			tele := telemetry.NewTelemetry()
			err := tele.Report(ctx, teleReport)
			if environment.DebugMode {
				if err != nil {
					log.Infof("report failed: %v", err)
				}
				fmt.Fprintf(os.Stderr, "report: %s\n", teleReport.String())
				if data, err := json.Marshal(teleReport); err == nil {
					log.Debugf("report: %s\n", string(data))
				}
			}
			cancel()
		}

		f()
	}

	color.Unset()

	if code != 0 {
		os.Exit(code)
	}
}
