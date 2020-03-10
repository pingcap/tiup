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

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var all, nightly, force, self bool
	cmd := &cobra.Command{
		Use:   "update [component1]:[version] [component2..N]",
		Short: "Update tiup components to the latest version",
		Long: `Update some components to the latest version, you must use --nightly
explicitly to update to the latest nightly version. You can use --all
to update all components installed locally. And you can specify a version
like <component>:<version> to update to the specified version. Some components
will be ignored if the latest version has been installed locally. But you can
use the flag --force explicitly to overwrite local installation. There is a
a flag --self, which is used to update the tiup to the latest version. All
other flags will be ignored if the flag --self specified.

  # Update all components to the latest stable version
  tiup update --all

  # Update all components to the latest nightly version
  tiup update --nightly --all

  # Overwrite the local installation
  tiup update playground:v0.0.3 --force

  # Update the tiup to the latest version
  tiup update --self`,
		RunE: func(cmd *cobra.Command, components []string) error {
			if self {
				originFile := filepath.Join(profile.Path("bin"), "tiup")
				renameFile := filepath.Join(profile.Path("bin"), "tiup.tmp")
				if err := os.Rename(originFile, renameFile); err != nil {
					return err
				}

				var err error
				defer func() {
					if err != nil {
						if err := os.Rename(renameFile, originFile); err != nil {
							fmt.Printf("Please rename `%s` to `%s` maunally\n", renameFile, originFile)
						}
					}
				}()
				err = repository.DownloadFile(profile.Path("bin"), "tiup")
				return err
			}
			if (len(components) == 0 && !all && !force) || (len(components) > 0 && all) {
				return cmd.Help()
			}
			return updateComponents(components, nightly, force)
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
		installed, err := profile.InstalledComponents()
		if err != nil {
			return err
		}
		components = installed
	}

	compDir := profile.ComponentsDir()
	manifest, err := repository.Manifest()
	if err != nil {
		return err
	}
	for _, comp := range components {
		component, version := meta.ParseCompVersion(comp)
		if !manifest.HasComponent(component) {
			return errors.Errorf("component `%s` not found", component)
		}
		manifest, err := repository.ComponentVersions(component)
		if err != nil {
			return err
		}
		err = profile.SaveVersions(component, manifest)
		if err != nil {
			return err
		}

		if nightly && manifest.Nightly == nil {
			fmt.Printf("The component `%s` has not nightly version, skiped\n", component)
			continue
		}

		if nightly {
			version = meta.NightlyVersion
			comp = fmt.Sprintf("%v:%v", component, version)
		}

		// Ignore if the version has been installed
		if !nightly && !force {
			versions, err := profile.InstalledVersions(component)
			if err != nil {
				return err
			}
			if version.IsEmpty() {
				version = manifest.LatestVersion()
			}
			var found bool
			for _, v := range versions {
				if meta.Version(v) == version {
					found = true
					break
				}
			}
			if found {
				continue
			}
		}
		err = repository.DownloadComponent(compDir, comp)
		if err != nil {
			return err
		}
	}
	return nil
}
