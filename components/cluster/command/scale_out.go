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
	"os"
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
		Use:          "scale-out <cluster-name> [topology.yaml]",
		Short:        "Scale out a TiDB cluster",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				clusterName string
				topoFile    string
			)

			// tiup cluster scale-out --stage1 --stage2
			// is equivalent to
			// tiup cluster scale-out
			if opt.Stage1 && opt.Stage2 {
				opt.Stage1 = false
				opt.Stage2 = false
			}

			if opt.Stage2 && len(args) == 1 {
				clusterName = args[0]
			} else {
				if len(args) != 2 {
					return cmd.Help()
				}
				clusterName = args[0]
				topoFile = args[1]
			}

			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			// stage2: topoFile is ""
			if data, err := os.ReadFile(topoFile); err == nil {
				teleTopology = string(data)
			}

			return cm.ScaleOut(
				clusterName,
				topoFile,
				postScaleOutHook,
				final,
				opt,
				skipConfirm,
				gOpt,
			)
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
	cmd.Flags().BoolVarP(&opt.SkipCreateUser, "skip-create-user", "", false, "(EXPERIMENTAL) Skip creating the user specified in topology.")
	cmd.Flags().StringVarP(&opt.IdentityFile, "identity_file", "i", opt.IdentityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.UsePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")
	cmd.Flags().BoolVarP(&opt.NoLabels, "no-labels", "", false, "Don't check TiKV labels")
	cmd.Flags().BoolVarP(&opt.Stage1, "stage1", "", false, "Don't start the new instance after scale-out, need to manually execute cluster scale-out --stage2")
	cmd.Flags().BoolVarP(&opt.Stage2, "stage2", "", false, "Start the new instance and init config after scale-out --stage1")

	return cmd
}

func final(builder *task.Builder, name string, meta spec.Metadata, gOpt operator.Options) {
	builder.UpdateTopology(name,
		tidbSpec.Path(name),
		meta.(*spec.ClusterMeta),
		nil, /* deleteNodeIds */
	)
}

func postScaleOutHook(builder *task.Builder, newPart spec.Topology, gOpt operator.Options) {
	postDeployHook(builder, newPart, gOpt)
}
