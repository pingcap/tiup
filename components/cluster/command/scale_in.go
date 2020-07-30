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
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			scale := func(b *task.Builder, imetadata spec.Metadata) {
				metadata := imetadata.(*spec.ClusterMeta)
				if !gOpt.Force {
					b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, gOpt).
						UpdateMeta(clusterName, metadata, operator.AsyncNodes(metadata.Topology, gOpt.Nodes, false)).
						UpdateTopology(clusterName, metadata, operator.AsyncNodes(metadata.Topology, gOpt.Nodes, false))
				} else {
					b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, gOpt).
						UpdateMeta(clusterName, metadata, gOpt.Nodes).
						UpdateTopology(clusterName, metadata, gOpt.Nodes)
				}
			}

			return manager.ScaleIn(
				clusterName,
				skipConfirm,
				gOpt.SSHTimeout,
				gOpt.NativeSSH,
				gOpt.Force,
				gOpt.Nodes,
				scale,
			)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Specify the nodes")
	cmd.Flags().Int64Var(&gOpt.APITimeout, "transfer-timeout", 300, "Timeout in seconds when transferring PD and TiKV store leaders")
	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force just try stop and destroy instance before removing the instance from topo")

	_ = cmd.MarkFlagRequired("node")

	return cmd
}
