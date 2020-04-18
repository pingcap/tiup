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
	"runtime"

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

func newInstallCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <component1>[:version] [component2...N]",
		Short: "Install a specific version of a component",
		Long: `Install a specific version of a component. The component can be specified
by <component> or <component>:<version>. The latest stable version will
be installed if there is no version specified.

You can install multiple components at once, or install multiple versions
of the same component:

  tiup install tidb:v3.0.5 tikv pd
  tiup install tidb:v3.0.5 tidb:v3.0.8 tikv:v3.0.9`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return installComponents(env, args)
		},
	}
	return cmd
}

func installComponents(env *meta.Environment, specs []string) error {
	manifest, err := env.LatestManifest()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		component, version := meta.ParseCompVersion(spec)
		compInfo, found := manifest.FindComponent(component)
		if !found {
			return fmt.Errorf("component `%s` not exists", component)
		}

		if !compInfo.IsSupport(runtime.GOOS, runtime.GOARCH) {
			return fmt.Errorf("component `%s` does not support `%s/%s`", component, runtime.GOOS, runtime.GOARCH)
		}

		err := env.DownloadComponent(component, version, version.IsNightly())
		if err != nil {
			return err
		}
	}
	return nil
}
