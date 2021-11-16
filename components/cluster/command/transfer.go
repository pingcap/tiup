// Copyright 2021 PingCAP, Inc.
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
	"github.com/pingcap/tiup/pkg/cluster/manager"
	"github.com/spf13/cobra"
)

/* Add a pair of adb like commands to transfer files to or from remote
   servers. Not using `scp` as the real implementation is not necessarily
   SSH, not using `transfer` all-in-one command to get rid of complex
   checking of wheather a path is remote or local, as this is supposed
   to be only a tiny helper utility.
*/

func newPullCmd() *cobra.Command {
	opt := manager.TransferOptions{Pull: true}
	cmd := &cobra.Command{
		Use:    "pull <cluster-name> <remote-path> <local-path>",
		Short:  "(EXPERIMENTAL) Transfer files or directories from host in the tidb cluster to local",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 3 {
				return cmd.Help()
			}

			clusterName := args[0]
			opt.Remote = args[1]
			opt.Local = args[2]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			return cm.Transfer(clusterName, opt, gOpt)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only exec on host with specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only exec on host with specified nodes")
	cmd.Flags().IntVarP(&opt.Limit, "limit", "l", 0, "Limits the used bandwidth, specified in Kbit/s")
	cmd.Flags().BoolVar(&opt.Compress, "compress", false, "Compression enable. Passes the -C flag to ssh(1) to enable compression.")

	return cmd
}

func newPushCmd() *cobra.Command {
	opt := manager.TransferOptions{Pull: false}
	cmd := &cobra.Command{
		Use:    "push <cluster-name> <local-path> <remote-path>",
		Short:  "(EXPERIMENTAL) Transfer files or directories from local to host in the tidb cluster",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 3 {
				return cmd.Help()
			}

			clusterName := args[0]
			opt.Local = args[1]
			opt.Remote = args[2]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			return cm.Transfer(clusterName, opt, gOpt)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only exec on host with specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only exec on host with specified nodes")

	return cmd
}
