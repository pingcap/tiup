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
	"path/filepath"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var all, nightly, force, self bool
	cmd := &cobra.Command{
		Use:   "update [component1][:version] [component2..N]",
		Short: "Update tiup components to the latest version",
		Long: `Update some components to the latest version. Use --nightly
to update to the latest nightly version. Use --all to update all components 
installed locally. Use <component>:<version> to update to the specified 
version. Components will be ignored if the latest version has already been 
installed locally, but you can use --force explicitly to overwrite an 
existing installation. Use --self which is used to update TiUP to the 
latest version. All other flags will be ignored if the flag --self is given.

  $ tiup update --all                     								# Update all components to the latest stable version of all mirrors
  $ tiup update --nightly --all           								# Update all components to the latest nightly version of all mirrors
  $ tiup update playground 												# Update the specific component to the latest stable version of default mirror
  $ tiup update playground:v0.0.3 --force			 					# Overwrite an existing local installation of default mirror
  $ tiup update tiup-mirrors.pingcap.com/playground 					# Update the specific component to the latest stable version of a specific mirror
  $ tiup update tiup-mirrors.pingcap.com/playground:v0.0.3 --force		# Overwrite an existing local installation of default mirror
  $ tiup update --self                    								# Update TiUP to the latest version of default mirror`,
		RunE: func(cmd *cobra.Command, components []string) error {
			teleCommand = cmd.CommandPath()
			if (len(components) == 0 && !all && !force && !self) || (len(components) > 0 && all) {
				return cmd.Help()
			}

			if self {
				if err := checkTiUPBinary(); err != nil {
					return err
				}

				originFile := filepath.Join(tiupC.TiUPHomePath(), "bin", "tiup")
				renameFile := filepath.Join(tiupC.TiUPHomePath(), "bin", "tiup.tmp")
				if err := os.Rename(originFile, renameFile); err != nil {
					fmt.Printf("Backup of `%s` to `%s` failed.\n", originFile, renameFile)
					return err
				}

				var err error
				defer func() {
					if err != nil || utils.IsNotExist(originFile) {
						if err := os.Rename(renameFile, originFile); err != nil {
							fmt.Printf("Please rename `%s` to `%s` manually.\n", renameFile, originFile)
						}
					} else {
						if err := os.Remove(renameFile); err != nil {
							fmt.Printf("Please delete `%s` manually.\n", renameFile)
						}
					}
				}()

				err = tiupC.SelfUpdate()
				if err != nil {
					return err
				}
			}
			if force || all || len(components) > 0 {
				err := updateComponents(components, nightly, force)
				if err != nil {
					return err
				}
			}
			fmt.Println("Updated successfully!")
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Update all components")
	cmd.Flags().BoolVar(&nightly, "nightly", false, "Update the components to nightly version")
	cmd.Flags().BoolVar(&force, "force", false, "Force update a component to the latest version")
	cmd.Flags().BoolVar(&self, "self", false, "Update tiup to the latest version")
	return cmd
}

func updateComponents(components []string, nightly, force bool) error {
	//env := environment.GlobalEnv()

	if len(components) == 0 {
		for _, repo := range tiupC.Repositories() {
			installed, err := repo.Local().InstalledComponentsWithMirror()
			if err != nil {
				return err
			}
			components = append(components, installed...)
		}

	}

	return tiupC.DownloadComponents(components, nightly, force)
}

// checkTiUPBinary check if TiUP exists in TiUP_HOME
func checkTiUPBinary() error {
	tiUPHomePath, _ := filepath.Abs(filepath.Join(tiupC.TiUPHomePath(), "bin", "tiup"))

	realTiUPPath, err := os.Executable()
	if err != nil {
		// Ignore the problem that the execution directory cannot be obtained
		return nil
	}
	realTiUPPath, _ = filepath.Abs(realTiUPPath)

	if utils.IsNotExist(tiUPHomePath) || tiUPHomePath != realTiUPPath {
		fmt.Printf("Tiup install directory is: %s\n", filepath.Dir(realTiUPPath))
		return fmt.Errorf("If you used some external package manager to install TiUP (e.g., brew), try upgrade with that")
	}

	return nil
}
