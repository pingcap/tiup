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

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <component1>[:version] [component2...N]",
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
	manifest, err := meta.LatestManifest()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		component, version := meta.ParseCompVersion(spec)
		if !manifest.HasComponent(component) {
			return fmt.Errorf("component `%s` does not support", component)
		}
		err := meta.DownloadComponent(component, version, version.IsNightly())
		if err != nil {
			return err
		}
	}
	return nil
}
