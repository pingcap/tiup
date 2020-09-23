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
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/spf13/cobra"
)

func newDestroyCmd() *cobra.Command {
	destoyOpt := operator.Options{}
	cmd := &cobra.Command{
		Use:   "destroy <cluster-name>",
		Short: "Destroy a specified cluster",
		Long: `Destroy a specified cluster, which will clean the deployment binaries and data.
You can retain some nodes and roles data when destroy cluster, eg:

  $ tiup cluster destroy <cluster-name> --retain-role-data prometheus
  $ tiup cluster destroy <cluster-name> --retain-node-data 172.16.13.11:9000
  $ tiup cluster destroy <cluster-name> --retain-node-data 172.16.13.12`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			// Validate the retained roles to prevent unexpected deleting data
			if len(destoyOpt.RetainDataRoles) > 0 {
				validRoles := set.NewStringSet(spec.AllComponentNames()...)
				for _, role := range destoyOpt.RetainDataRoles {
					if !validRoles.Exist(role) {
						return perrs.Errorf("role name `%s` invalid", role)
					}
				}
			}

			return manager.DestroyCluster(clusterName, gOpt, destoyOpt, skipConfirm)
		},
	}

	cmd.Flags().StringArrayVar(&destoyOpt.RetainDataNodes, "retain-node-data", nil, "Specify the nodes or hosts whose data will be retained")
	cmd.Flags().StringArrayVar(&destoyOpt.RetainDataRoles, "retain-role-data", nil, "Specify the roles whose data will be retained")
	cmd.Flags().BoolVar(&destoyOpt.Force, "force", false, "Force will ignore any error while destroy the cluster")

	return cmd
}
