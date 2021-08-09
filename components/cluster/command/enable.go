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

func newEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <cluster-name>",
		Short: "Enable a TiDB cluster automatically at boot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return nil
			}

			clusterName := args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			return cm.EnableCluster(clusterName, gOpt, true)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only enable specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only enable specified nodes")

	return cmd
}
