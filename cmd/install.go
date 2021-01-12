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

	"github.com/pingcap/tiup/pkg/environment"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <component1>[:version] [component2...N]",
		Short: "Install a specific version of a component",
		Long: fmt.Sprintf(`Install a specific version of a component. The component can be specified
by <component> or <component>:<version>. The latest stable version will
be installed if there is no version specified.

You can install multiple components at once, or install multiple versions
of the same component:

  %[1]s install tidb:v4.0.8 tikv pd
  %[1]s install tidb:v4.0.7 tidb:v4.0.8 tikv:v4.0.9`, brand),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := environment.GlobalEnv()
			if len(args) == 0 {
				return cmd.Help()
			}
			return installComponents(env, args)
		},
	}
	return cmd
}

func installComponents(env *environment.Environment, specs []string) error {
	return env.UpdateComponents(specs, false, false)
}
