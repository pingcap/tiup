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
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {
	cobra.EnableCommandSorting = false

	var (
		binary   string
		tag      string
		rm       bool
		repoOpts repository.Options
	)

	rootCmd = &cobra.Command{
		Use: `tiup [flags] <command> [args...]
  tiup [flags] <component> [args...]`,
		Long: `The tiup is a component management CLI utility tool that can help to download and install
the TiDB components to the local system. You can run a specific version of a component via
"tiup <component>[:version]". If no version number is specified, the latest version installed
locally will be run. If the specified component does not have any version installed locally,
the latest stable version will be downloaded from the repository.

  # *HOW TO* reuse instance data instead of generating a new data directory each time?
  # The instances which have the same "TAG" will share the data directory: $TIUP_HOME/data/$TAG.
  $ tiup --tag mycluster playground`,
		SilenceErrors:      true,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		Version:            fmt.Sprintf("%s+%s(%s)", version.NewTiUPVersion().SemVer(), version.GitBranch, version.GitHash),
		Args: func(cmd *cobra.Command, args []string) error {
			// Support `tiup <component>:[<version>]:[<binpath>]`
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if binary != "" {
				component, ver, binPath := meta.ParseBinary(binary)
				selectedVer, err := meta.SelectInstalledVersion(component, ver)
				if err != nil {
					return err
				}
				if binPath == "" {
					binPath, err = meta.BinaryPath(component, selectedVer)
					if err != nil {
						return err
					}
				}

				fmt.Println(binPath)
				return nil
			}
			if len(args) > 0 {
				// We assume the first unknown parameter is the component name and following
				// parameters will be transparent passed because registered flags and subcommands
				// will be parsed correctly.
				// e.g: tiup --tag mytag --rm playground --db 3 --pd 3 --kv 4
				//   => run "playground" with parameters "--db 3 --pd 3 --kv 4"
				var transparentParams []string
				componentSpec := args[0]
				for i, arg := range os.Args {
					if arg == componentSpec {
						transparentParams = os.Args[i+1:]
						break
					}
				}
				return runComponent(tag, componentSpec, transparentParams, rm)
			}
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return meta.InitRepository(repoOpts)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return meta.Repository().Mirror().Close()
		},
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().BoolVarP(&repoOpts.SkipVersionCheck, "skip-version-check", "", false, "Skip the strict version check, by default a version must be a valid SemVer string")
	rootCmd.Flags().StringVarP(&binary, "binary", "B", "", "Print binary path of a specific version of a component `<component>[:version]`\n"+
		"and the latest version installed will be selected if no version specified")
	rootCmd.Flags().StringVarP(&tag, "tag", "T", "", "Specify a tag for component instance")
	rootCmd.Flags().BoolVar(&rm, "rm", false, "Remove the data directory when the component instance finishes its run")

	rootCmd.AddCommand(
		newInstallCmd(),
		newListCmd(),
		newUninstallCmd(),
		newUpdateCmd(),
		newStatusCmd(),
		newCleanCmd(),
	)

	originHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			originHelpFunc(cmd, args)
			return
		}
		cmd, n, e := cmd.Root().Find(args)
		if (cmd == rootCmd || e != nil) && len(n) > 0 {
			externalHelp(n[0], n[1:]...)
		} else {
			cmd.InitDefaultHelpFlag() // make possible 'help' flag to be shown
			cmd.Help()
		}
	})
	rootCmd.SetHelpCommand(newHelpCmd())
}

// Execute parses the command line arguments and calls proper functions
func Execute() {
	rootCmd.SetUsageTemplate(usageTemplate())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}
}
