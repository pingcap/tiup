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
	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type execOptions struct {
	command string
	sudo    bool
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
			teleCommand = append(teleCommand, scrubClusterName(clusterName))
			if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return errors.Errorf("cannot execute command on non-exists cluster %s", clusterName)
			}

			logger.EnableAuditLog()
			metadata, err := meta.ClusterMetadata(clusterName)
			if err != nil {
				return err
			}

			filterRoles := set.NewStringSet(gOpt.Roles...)
			filterNodes := set.NewStringSet(gOpt.Nodes...)

			var shellTasks []task.Task
			uniqueHosts := map[string]int{} // host -> ssh-port
			metadata.Topology.IterInstance(func(inst meta.Instance) {
				if _, found := uniqueHosts[inst.GetHost()]; !found {
					if len(gOpt.Roles) > 0 && !filterRoles.Exist(inst.Role()) {
						return
					}

					if len(gOpt.Nodes) > 0 && !filterNodes.Exist(inst.GetHost()) {
						return
					}

					uniqueHosts[inst.GetHost()] = inst.GetSSHPort()
				}
			})

			for host := range uniqueHosts {
				shellTasks = append(shellTasks,
					task.NewBuilder().
						Shell(host, opt.command, opt.sudo).
						Build())
			}

			t := task.NewBuilder().
				SSHKeySet(
					meta.ClusterPath(clusterName, "ssh", "id_rsa"),
					meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
				ClusterSSH(metadata.Topology, metadata.User, gOpt.SSHTimeout).
				Parallel(shellTasks...).
				Build()

			execCtx := task.NewContext()
			if err := t.Execute(execCtx); err != nil {
				if errorx.Cast(err) != nil {
					// FIXME: Map possible task errors and give suggestions.
					return err
				}
				return errors.Trace(err)
			}

			// print outputs
			for host := range uniqueHosts {
				stdout, stderr, ok := execCtx.GetOutputs(host)
				if !ok {
					continue
				}
				log.Infof("Outputs of %s on %s:",
					color.CyanString(opt.command),
					color.CyanString(host))
				if len(stdout) > 0 {
					log.Infof("%s:\n%s", color.GreenString("stdout"), stdout)
				}
				if len(stderr) > 0 {
					log.Infof("%s:\n%s", color.RedString("stderr"), stderr)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opt.command, "command", "ls", "the command run on cluster host")
	cmd.Flags().BoolVar(&opt.sudo, "sudo", false, "use root permissions (default false)")
	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only exec on host with specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only exec on host with specified nodes")

	return cmd
}
