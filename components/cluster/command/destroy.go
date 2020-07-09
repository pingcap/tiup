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
	"errors"
	"os"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/spf13/cobra"
)

func newDestroyCmd() *cobra.Command {
	destoyOpt := operator.Options{}
	cmd := &cobra.Command{
		Use: "destroy <cluster-name>",
		Long: `Destroy a specified cluster, which will clean the deployment binaries and data.
You can retain some nodes and roles data when destroy cluster, eg:
  
  $ tiup cluster destroy <cluster-name> --retain-role-data prometheus
  $ tiup cluster destroy <cluster-name> --retain-node-data 172.16.13.11:9000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			// Validate the retained roles to prevent unexpected deleting data
			if len(destoyOpt.RetainDataRoles) > 0 {
				validRoles := set.NewStringSet(spec.AllComponentNames()...)
				for _, role := range destoyOpt.RetainDataRoles {
					if !validRoles.Exist(role) {
						return perrs.Errorf("role name `%s` invalid", role)
					}
				}
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			exist, err := tidbSpec.Exist(clusterName)
			if err != nil {
				return perrs.AddStack(err)
			}

			if !exist {
				return perrs.Errorf("cannot destroy non-exists cluster %s", clusterName)
			}

			logger.EnableAuditLog()
			metadata, err := spec.ClusterMetadata(clusterName)
			if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
				perrs.Cause(err) != spec.ErrNoTiSparkMaster {
				return err
			}

			if !skipConfirm {
				if err := cliutil.PromptForConfirmOrAbortError(
					"This operation will destroy TiDB %s cluster %s and its data.\nDo you want to continue? [y/N]:",
					color.HiYellowString(metadata.Version),
					color.HiYellowString(clusterName)); err != nil {
					return err
				}
				log.Infof("Destroying cluster...")
			}

			t := task.NewBuilder().
				SSHKeySet(
					spec.ClusterPath(clusterName, "ssh", "id_rsa"),
					spec.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
				ClusterSSH(metadata.Topology, metadata.User, gOpt.SSHTimeout).
				ClusterOperate(metadata.Topology, operator.StopOperation, operator.Options{}).
				ClusterOperate(metadata.Topology, operator.DestroyOperation, destoyOpt).
				Build()

			if err := t.Execute(task.NewContext()); err != nil {
				if errorx.Cast(err) != nil {
					// FIXME: Map possible task errors and give suggestions.
					return err
				}
				return perrs.Trace(err)
			}

			if err := os.RemoveAll(spec.ClusterPath(clusterName)); err != nil {
				return perrs.Trace(err)
			}
			log.Infof("Destroyed cluster `%s` successfully", clusterName)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&destoyOpt.RetainDataNodes, "retain-node-data", nil, "Specify the nodes whose data will be retained")
	cmd.Flags().StringArrayVar(&destoyOpt.RetainDataRoles, "retain-role-data", nil, "Specify the roles whose data will be retained")

	return cmd
}
