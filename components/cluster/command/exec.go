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
	"github.com/pingcap/tiup/pkg/cluster"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	opt := cluster.ExecOptions{}
	cmd := &cobra.Command{
		Use:   "exec <cluster-name>",
		Short: "Run shell command on host in the tidb cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			return manager.Exec(clusterName, opt, gOpt)
		},
	}

	cmd.Flags().StringVar(&opt.Command, "command", "ls", "the command run on cluster host")
	cmd.Flags().BoolVar(&opt.Sudo, "sudo", false, "use root permissions (default false)")
	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only exec on host with specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only exec on host with specified nodes")

	return cmd
}
