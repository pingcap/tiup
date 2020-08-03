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
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cleanOpt := operator.Options{}
	withLog := false

	cmd := &cobra.Command{
		Use:  "clean <cluster-name>",
		Long: `clean cluster data without destroy cluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			return manager.CleanCluster(clusterName, gOpt, cleanOpt, skipConfirm, withLog)
		},
	}

	cmd.Flags().StringArrayVar(&cleanOpt.RetainDataNodes, "retain-node-data", nil, "Specify the nodes whose data will be retained")
	cmd.Flags().StringArrayVar(&cleanOpt.RetainDataRoles, "retain-role-data", nil, "Specify the roles whose data will be retained")
	cmd.Flags().BoolVar(&withLog, "with-log", false, "Cleanup log too")

	return cmd
}
