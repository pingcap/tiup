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
	"os/user"
	"path/filepath"
	"sort"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/version"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var (
	profile    *localdata.Profile
	rootCmd    *cobra.Command
	repository *meta.Repository
)

const defaultMirror = "https://tiup-mirrors.pingcap.com/"

var mirrorRepository = ""

func init() {
	cobra.EnableCommandSorting = false

	var (
		mirror   = defaultMirror
		binary   string
		tag      string
		rm       bool
		repoOpts meta.RepositoryOptions
	)
	if m := os.Getenv("TIUP_MIRRORS"); m != "" {
		mirror = m
	}

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
			// Support `tiup <component>`
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if binary != "" {
				binaryPath, err := binaryPath(binary)
				if err != nil {
					return err
				}
				fmt.Println(binaryPath)
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
			// Initialize the repository
			// Replace the mirror if some sub-commands use different mirror address
			mirror := meta.NewMirror(mirrorRepository)
			if err := mirror.Open(); err != nil {
				return err
			}
			repository = meta.NewRepository(mirror, repoOpts)
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return repository.Mirror().Close()
		},
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVarP(&mirrorRepository, "mirror", "", mirror, "Overwrite default `mirror` or TIUP_MIRRORS environment variable")
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
			externalHelp(n[0])
		} else {
			cmd.InitDefaultHelpFlag() // make possible 'help' flag to be shown
			cmd.Help()
		}
	})
	rootCmd.SetHelpCommand(newHelpCmd())
}

func componentAndVersion(spec string) (string, meta.Version, error) {
	component, version := meta.ParseCompVersion(spec)
	installed, err := profile.InstalledVersions(component)
	if err != nil {
		return "", "", err
	}

	errInstallFirst := fmt.Errorf("use `tiup install %[1]s` to install `%[1]s` first", spec)
	if len(installed) < 1 {
		return "", "", errInstallFirst
	}
	if version.IsEmpty() {
		sort.Slice(installed, func(i, j int) bool {
			return semver.Compare(installed[i], installed[j]) < 0
		})
		version = meta.Version(installed[len(installed)-1])
	}
	found := false
	for _, v := range installed {
		if meta.Version(v) == version {
			found = true
			break
		}
	}
	if !found {
		return "", "", errInstallFirst
	}
	return component, version, nil
}

func binaryPath(spec string) (string, error) {
	component, version, err := componentAndVersion(spec)
	if err != nil {
		return "", err
	}
	return profile.BinaryPath(component, version)
}

func installPath(spec string) (string, error) {
	component, version, err := componentAndVersion(spec)
	if err != nil {
		return "", err
	}
	return profile.ComponentInstallPath(component, version)
}

func execute() error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	// Initialize the global profile
	var profileDir string
	switch {
	case os.Getenv(localdata.EnvNameHome) != "":
		profileDir = os.Getenv(localdata.EnvNameHome)
	case localdata.DefaultTiupHome != "":
		profileDir = localdata.DefaultTiupHome
	default:
		profileDir = filepath.Join(u.HomeDir, localdata.ProfileDirName)
	}
	profile = localdata.NewProfile(profileDir)

	rootCmd.SetUsageTemplate(usageTemplate())
	return rootCmd.Execute()
}

// Execute parses the command line arguments and calls proper functions
func Execute() {
	if err := execute(); err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}
}
