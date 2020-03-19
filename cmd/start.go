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
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/topology"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var (
		clusterName string
		role        string
		node        string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {

			var spec *topology.Specification
			// TODO get by cluster name

			t := task.NewBuilder().
				ClusterSSH(spec).
				ClusterOperate(spec, "start", role, node).
				Build()

			return t.Execute(task.NewContext())

		},
	}

	cmd.Flags().StringVar(&clusterName, "cluster_name", "", "cluster name")
	cmd.Flags().StringVar(&role, "role", "", "role name")
	cmd.Flags().StringVar(&node, "node-id", "", "node id")
	return cmd
}
