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

	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <component1>:[version] [component2...N]",
		Short: "Install a specific version of a component",
		Long: `Install a specific version of a component, the component can be specified
by <component> or <component>:<version> and the latest stable version will
be installed if there is no version part specified.

You can install multiple components at once, or install multiple versions
of the same component:

  tiup install tidb:v3.0.5 tikv pd
  tiup install tidb:v3.0.5 tidb:v3.0.8 tikv:v3.0.9`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return installComponents(args)
		},
	}
	return cmd
}

func installComponents(specs []string) error {
	for _, spec := range specs {
		manifest, err := repository.Manifest()
		if err != nil {
			return err
		}
		component, version := meta.ParseCompVersion(spec)
		if !manifest.HasComponent(component) {
			return fmt.Errorf("component `%s` does not support", component)
		}

		versions, err := repository.ComponentVersions(component)
		if err != nil {
			return err
		}

		err = profile.SaveVersions(component, versions)
		if err != nil {
			return err
		}

		if !version.IsNightly() {
			// Ignore if installed
			installed, err := profile.InstalledVersions(component)
			if err != nil {
				return err
			}
			if version.IsEmpty() {
				version = versions.LatestVersion()
			}
			found := false
			for _, v := range installed {
				if meta.Version(v) == version {
					found = true
					break
				}
			}
			if found {
				fmt.Printf("The `%s:%s` has been installed\n", component, version)
				continue
			}
		}

		compDir := profile.ComponentsDir()
		err = repository.DownloadComponent(compDir, spec)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
