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
	perrs "github.com/pingcap/errors"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cleanOpt := operator.Options{}
	cleanALl := false

	cmd := &cobra.Command{
		Use:  "clean <cluster-name>",
		Long: `clean cluster data without destroy cluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			if cleanALl {
				cleanOpt.CleanupData = true
				cleanOpt.CleanupLog = true
			}

			if !(cleanOpt.CleanupData || cleanOpt.CleanupLog) {
				return perrs.Errorf("at least one of `--all` `--data` `--log` should be specified")
			}

			return manager.CleanCluster(clusterName, gOpt, cleanOpt, skipConfirm)
		},
	}

	cmd.Flags().StringArrayVar(&cleanOpt.RetainDataNodes, "ignore-node", nil, "Specify the nodes whose data will be retained")
	cmd.Flags().StringArrayVar(&cleanOpt.RetainDataRoles, "ignore-role", nil, "Specify the roles whose data will be retained")
	cmd.Flags().BoolVar(&cleanOpt.CleanupData, "data", false, "Cleanup data")
	cmd.Flags().BoolVar(&cleanOpt.CleanupLog, "log", false, "Cleanup log")
	cmd.Flags().BoolVar(&cleanALl, "all", false, "Cleanup both log and data")

	return cmd
}
