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

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var all, nightly, force, self bool
	cmd := &cobra.Command{
		Use:   "update [component1][:version] [component2..N]",
		Short: "Update tiup components to the latest version",
		Long: `Update some components to the latest version, you must use --nightly
explicitly to update to the latest nightly version. You can use --all
to update all components installed locally. And you can specify a version
like <component>:<version> to update to the specified version. Some components
will be ignored if the latest version has been installed locally. But you can
use the flag --force explicitly to overwrite local installation. There is a
a flag --self, which is used to update the tiup to the latest version. All
other flags will be ignored if the flag --self specified.

  $ tiup update --all                     # Update all components to the latest stable version
  $ tiup update --nightly --all           # Update all components to the latest nightly version
  $ tiup update playground:v0.0.3 --force # Overwrite the local installation
  $ tiup update --self                    # Update the tiup to the latest version`,
		RunE: func(cmd *cobra.Command, components []string) error {
			if self {
				originFile := meta.LocalPath("bin", "tiup")
				renameFile := meta.LocalPath("bin", "tiup.tmp")
				if err := os.Rename(originFile, renameFile); err != nil {
					fmt.Printf("Backup `%s` to `%s` failed\n", originFile, renameFile)
					return err
				}

				var err error
				defer func() {
					if err != nil {
						if err := os.Rename(renameFile, originFile); err != nil {
							fmt.Printf("Please rename `%s` to `%s` maunally\n", renameFile, originFile)
						}
					} else {
						if err := os.Remove(renameFile); err != nil {
							fmt.Printf("Please delete `%s` maunally\n", renameFile)
						}
					}
				}()

				err = meta.Repository().DownloadFile(meta.LocalPath("bin"), "tiup")
				if err != nil {
					return err
				}
				fmt.Println("Update successfully!")
				return nil
			}
			if (len(components) == 0 && !all && !force) || (len(components) > 0 && all) {
				return cmd.Help()
			}
			err := updateComponents(components, nightly, force)
			if err != nil {
				return err
			}
			fmt.Println("Update successfully!")
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
	if len(components) == 0 {
		installed, err := meta.Profile().InstalledComponents()
		if err != nil {
			return err
		}
		components = installed
	}

	manifest, err := meta.LatestManifest()
	if err != nil {
		return err
	}
	for _, comp := range components {
		component, version := meta.ParseCompVersion(comp)
		if !manifest.HasComponent(component) {
			return errors.Errorf("component `%s` not found", component)
		}
		if nightly {
			version = repository.NightlyVersion
		}
		err = meta.DownloadComponent(component, version, nightly || force)
		if err != nil {
			return err
		}
	}
	return nil
}
