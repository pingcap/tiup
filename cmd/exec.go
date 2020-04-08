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
	"sort"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/logger"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/task"
	"github.com/pingcap-incubator/tiup/pkg/set"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type execOptions struct {
	command string
	sudo    bool
	roles   []string
	nodes   []string
}

func newExecCmd() *cobra.Command {
	opt := execOptions{}
	cmd := &cobra.Command{
		Use:   "exec <cluster-name>",
		Short: "Run shell command on host in the tidb cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return errors.Errorf("cannot execute command on non-exists cluster %s", clusterName)
			}

			logger.EnableAuditLog()
			metadata, err := meta.ClusterMetadata(clusterName)
			if err != nil {
				return err
			}

			var hosts []string
			filterRoles := set.NewStringSet(opt.roles...)
			filterNodes := set.NewStringSet(opt.nodes...)

			var shellTasks []task.Task
			metadata.Topology.IterInstance(func(inst meta.Instance) {
				h := inst.GetHost()
				if len(opt.roles) > 0 && !filterRoles.Exist(inst.Role()) {
					return
				}

				if len(opt.nodes) > 0 && !filterNodes.Exist(h) {
					return
				}

				hosts = append(hosts, h)
			})

			sort.Strings(hosts)
			for i := 0; i < len(hosts); i++ {
				if i > 0 && hosts[i] == hosts[i-1] {
					continue
				}
				shellTasks = append(shellTasks, task.NewBuilder().Shell(hosts[i], opt.command, opt.sudo).Build())
			}

			t := task.NewBuilder().
				SSHKeySet(
					meta.ClusterPath(clusterName, "ssh", "id_rsa"),
					meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
				ClusterSSH(metadata.Topology, metadata.User, sshTimeout).
				Parallel(shellTasks...).
				Build()

			if err := t.Execute(task.NewContext()); err != nil {
				if errorx.Cast(err) != nil {
					// FIXME: Map possible task errors and give suggestions.
					return err
				}
				return errors.Trace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opt.command, "command", "ls", "the command run on cluster host")
	cmd.Flags().BoolVar(&opt.sudo, "sudo", false, "use root permissions (default false)")
	cmd.Flags().StringSliceVarP(&opt.roles, "role", "R", nil, "Only exec on host with specified roles")
	cmd.Flags().StringSliceVarP(&opt.nodes, "node", "N", nil, "Only exec on host with specified nodes")

	return cmd
}
