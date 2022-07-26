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
	"crypto/tls"

	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/spf13/cobra"
)

func newScaleInCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale-in <cluster-name>",
		Short: "Scale in a TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			scale := func(b *task.Builder, imetadata spec.Metadata, tlsCfg *tls.Config) {
				metadata := imetadata.(*spec.ClusterMeta)

				nodes := gOpt.Nodes
				if !gOpt.Force {
					nodes = operator.AsyncNodes(metadata.Topology, nodes, false)
				}

				b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, gOpt, tlsCfg).
					UpdateMeta(clusterName, metadata, nodes).
					UpdateTopology(clusterName, tidbSpec.Path(clusterName), metadata, nodes)
			}

			return cm.ScaleIn(clusterName, skipConfirm, gOpt, scale)
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

	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Specify the nodes (required)")
	cmd.Flags().Uint64Var(&gOpt.APITimeout, "transfer-timeout", 600, "Timeout in seconds when transferring PD and TiKV store leaders, also for TiCDC drain one capture")
	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force just try stop and destroy instance before removing the instance from topo")

	_ = cmd.MarkFlagRequired("node")

	return cmd
}
