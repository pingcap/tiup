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
	"path/filepath"

	"github.com/pingcap/tiup/pkg/cluster/manager"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newScaleOutCmd() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: filepath.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "scale-out <cluster-name> <topology.yaml>",
		Short:        "Scale out a DM cluster",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			clusterName := args[0]
			topoFile := args[1]

			return cm.ScaleOut(clusterName, topoFile, postScaleOutHook, nil, opt, skipConfirm, gOpt)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return shellCompGetClusterName(cm, toComplete)
			case 1:
				return nil, cobra.ShellCompDirectiveDefault
			default:
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.IdentityFile, "identity_file", "i", opt.IdentityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.UsePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")

	return cmd
}

func postScaleOutHook(builder *task.Builder, newPart spec.Topology, gOpt operator.Options) {
	postDeployHook(builder, newPart, gOpt)
}
