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

package command

import (
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	offlineMode := false
	ignoreVersionCheck := false
	cmd := &cobra.Command{
		Use:   "upgrade <cluster-name> <version>",
		Short: "Upgrade a specified DM cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			return cm.Upgrade(args[0], args[1], nil, gOpt, skipConfirm, offlineMode, ignoreVersionCheck)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return shellCompGetClusterName(cm, toComplete)
			default:
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		},
	}

	cmd.Flags().BoolVarP(&offlineMode, "offline", "", false, "Upgrade a stopped cluster")
	cmd.Flags().BoolVarP(&ignoreVersionCheck, "ignore-version-check", "", false, "Ignore checking if target version is higher than current version")

	return cmd
}
