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
	"path"

	"github.com/pingcap/tiup/pkg/cluster/manager"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	opt := manager.CheckOptions{
		Opr:          &operator.CheckOptions{},
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:   "check <topology.yml | cluster-name> [scale-out.yml]",
		Short: "Perform preflight checks for the cluster.",
		Long: `Perform preflight checks for the cluster. By default, it checks deploy servers
before a cluster is deployed, the input is the topology.yaml for the cluster.
If '--cluster' is set, it will perform checks for an existing cluster, the input
is the cluster name. Some checks are ignore in this mode, such as port and dir
conflict checks with other clusters
If you want to check the scale-out topology, please use execute the following command
'	check <cluster-name> <scale-out.yml> --cluster	'
it will check the new instances `,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 && len(args) != 2 {
				return cmd.Help()
			}
			scaleOutTopo := ""

			if opt.ExistCluster {
				clusterReport.ID = scrubClusterName(args[0])
			}

			if len(args) == 2 {
				if !opt.ExistCluster {
					return cmd.Help()
				}
				scaleOutTopo = args[1]
			}

			return cm.CheckCluster(args[0], scaleOutTopo, opt, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.IdentityFile, "identity_file", "i", opt.IdentityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.UsePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")
	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only check specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only check specified nodes")

	cmd.Flags().BoolVar(&opt.Opr.EnableCPU, "enable-cpu", false, "Enable CPU thread count check")
	cmd.Flags().BoolVar(&opt.Opr.EnableMem, "enable-mem", false, "Enable memory size check")
	cmd.Flags().BoolVar(&opt.Opr.EnableDisk, "enable-disk", false, "Enable disk IO (fio) check")
	cmd.Flags().BoolVar(&opt.ApplyFix, "apply", false, "Try to fix failed checks")
	cmd.Flags().BoolVar(&opt.ExistCluster, "cluster", false, "Check existing cluster, the input is a cluster name.")
	cmd.Flags().Uint64Var(&gOpt.APITimeout, "api-timeout", 10, "Timeout in seconds when querying PD APIs.")

	return cmd
}
