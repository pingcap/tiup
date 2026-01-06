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

	"github.com/pingcap/tiup/pkg/environment"
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

  $ tiup update --all                     # Update all components to the latest stable version
  $ tiup update --nightly --all           # Update all components to the latest nightly version
  $ tiup update playground:v0.0.3 --force # Overwrite an existing local installation
  $ tiup update --self                    # Update TiUP to the latest version`,
		RunE: func(cmd *cobra.Command, components []string) error {
			if (len(components) == 0 && !all && !force && !self) || (len(components) > 0 && all) {
				return cmd.Help()
			}

			env := environment.GlobalEnv()
			if self {
				if err := checkTiUPBinary(env); err != nil {
					return err
				}

				originFile := env.LocalPath("bin", "tiup")
				renameFile := env.LocalPath("bin", "tiup.tmp")
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

				err = env.SelfUpdate()
				if err != nil {
					return err
				}
			}
			if force || all || len(components) > 0 {
				err := updateComponents(env, components, nightly, force)
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

func updateComponents(env *environment.Environment, components []string, nightly, force bool) error {
	if len(components) == 0 {
		installed, err := env.Profile().InstalledComponents()
		if err != nil {
			return err
		}
		components = installed
	}

	return env.UpdateComponents(components, nightly, force)
}

// checkTiUPBinary check if TiUP exists in TiUP_HOME
func checkTiUPBinary(env *environment.Environment) error {
	tiUPHomePath, _ := filepath.Abs(env.LocalPath("bin", "tiup"))

	realTiUPPath, err := os.Executable()
	if err != nil {
		// Ignore the problem that the execution directory cannot be obtained
		return nil
	}
	realTiUPPath, _ = filepath.Abs(realTiUPPath)

	if utils.IsNotExist(tiUPHomePath) || tiUPHomePath != realTiUPPath {
		fmt.Printf("Tiup install directory is: %s\n", filepath.Dir(realTiUPPath))
		return fmt.Errorf("if you used some external package manager to install TiUP (e.g., brew), try upgrade with that")
	}

	return nil
}
