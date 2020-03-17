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

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiops/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {
	cobra.EnableCommandSorting = false

	rootCmd = &cobra.Command{
		Use:     "tiops",
		Short:   "Deploy a TiDB cluster for production",
		Version: version.NewTiOpsVersion().String(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.AddCommand(
		newDeploy(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newScaleInCmd(),
		newScaleOutCmd(),
		newDestroyCmd(),
		newUpgradeCmd(),
		newReloadCmd(),
		newExecCmd(),
	)
}

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}
}
