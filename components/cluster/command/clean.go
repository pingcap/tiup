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
	"fmt"

	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cleanOpt := operator.Options{}
	cleanALl := false

	cmd := &cobra.Command{
		Use:   "clean <cluster-name>",
		Short: "(EXPERIMENTAL) Cleanup a specified cluster",
		Long: fmt.Sprintf(`EXPERIMENTAL: This is an experimental feature, things may or may not work,
please backup your data before process.

Cleanup a specified cluster without destroying it.
You can retain some nodes and roles data when cleanup the cluster, eg:
    $ %[1]s cluster clean <cluster-name> --all
    $ %[1]s cluster clean <cluster-name> --log
    $ %[1]s cluster clean <cluster-name> --data
    $ %[1]s cluster clean <cluster-name> --all --ignore-role prometheus
    $ %[1]s cluster clean <cluster-name> --all --ignore-node 172.16.13.11:9000
    $ %[1]s cluster clean <cluster-name> --all --ignore-node 172.16.13.12`, tiupVer.LowerName()),
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
				return cmd.Help()
			}

			return cm.CleanCluster(clusterName, gOpt, cleanOpt, skipConfirm)
		},
	}

	cmd.Flags().StringArrayVar(&cleanOpt.RetainDataNodes, "ignore-node", nil, "Specify the nodes or hosts whose data will be retained")
	cmd.Flags().StringArrayVar(&cleanOpt.RetainDataRoles, "ignore-role", nil, "Specify the roles whose data will be retained")
	cmd.Flags().BoolVar(&cleanOpt.CleanupData, "data", false, "Cleanup data")
	cmd.Flags().BoolVar(&cleanOpt.CleanupLog, "log", false, "Cleanup log")
	cmd.Flags().BoolVar(&cleanALl, "all", false, "Cleanup both log and data")

	return cmd
}
