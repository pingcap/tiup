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
	"os"

	"github.com/pingcap/tiup/pkg/client"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	var force bool
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
			teleCommand = cmd.CommandPath()
			//env := environment.GlobalEnv()
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := client.NewTiUPClient(os.Getenv(localdata.EnvNameHome))
			if err != nil {
				return err
			}
			return c.Install(args[0])
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "If the specified version was already installed, force a reinstallation")
	return cmd
}
