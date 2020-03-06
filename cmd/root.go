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

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const profileDirName = ".tiup"

var (
	profile    *localdata.Profile
	rootCmd    *cobra.Command
	repository *meta.Repository
)

const defaultMirror = "https://tiup-mirrors.pingcap.com/"

var mirrorRepository = ""

func init() {
	var mirror = defaultMirror
	if m := os.Getenv("TIUP_MIRRORS"); m != "" {
		mirror = m
	}

	var binary string
	rootCmd = &cobra.Command{
		Use: "tiup",
		Long: `The tiup is a component management CLI utility tool that can help
to download and install the TiDB components to the local system.

In addition, there is a sub-command tiup run <component>:[version]
to help us start a component quickly. The tiup will download the
component which doesn't be installed or the specified version is
missing.

  # Quick start
  tiup run playground`,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if binary != "" {
				binaryPath, err := binaryPath(binary)
				if err != nil {
					return err
				}
				fmt.Println(binaryPath)
				return nil
			}
			return cmd.Help()
		},
		SilenceUsage: true,
	}

	rootCmd.Flags().StringVarP(&mirrorRepository, "mirror", "", mirror, "Overwrite default `mirror` or TIUP_MIRRORS environment variable")
	rootCmd.Flags().StringVarP(&binary, "bin", "", "", "Print binary path of a specific version of a component `<component>:[version]`\n"+
		"and the latest version installed will be selected if no version specified.")

	rootCmd.AddCommand(
		newInstallCmd(),
		newListCmd(),
		newUninstallCmd(),
		newUpdateCmd(),
		newRunCmd(),
		newVersionCmd(),
		newStatusCmd(),
		newCleanCmd(),
	)
}

func binaryPath(spec string) (string, error) {
	component, version := meta.ParseCompVersion(spec)
	installed, err := profile.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	errInstallFirst := fmt.Errorf("use `tiup install %[1]s` to install `%[1]s` first", spec)
	if len(installed) < 1 {
		return "", errInstallFirst
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
		return "", errInstallFirst
	}
	return profile.BinaryPath(component, version)
}

func execute() error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	// Initialize the global profile
	profile = localdata.NewProfile(filepath.Join(u.HomeDir, profileDirName))

	// Initialize the repository
	// Replace the mirror if some subcommands use different mirror address
	mirror := meta.NewMirror(mirrorRepository)
	if err := mirror.Open(); err != nil {
		return err
	}
	repository = meta.NewRepository(mirror)
	defer func() { _ = repository.Mirror().Close() }()

	return rootCmd.Execute()
}

// Execute parses the command line argumnts and calls proper functions
func Execute() {
	if err := execute(); err != nil {
		fmt.Printf("\x1b[0;31mError: %s\x1b[0m\n", err)
		os.Exit(1)
	}
}
