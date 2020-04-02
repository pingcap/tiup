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
	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiops/pkg/logger"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/task"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type execOptions struct {
	command string
	sudo    bool
	//role string
	//node string
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

			var shellTasks []task.Task
			metadata.Topology.IterInstance(func(inst meta.Instance) {
				t := task.NewBuilder().
					UserSSH(inst.GetHost(), metadata.User).
					Shell(inst.GetHost(), opt.command, opt.sudo).
					Build()
				shellTasks = append(shellTasks, t)
			})

			t := task.NewBuilder().
				SSHKeySet(
					meta.ClusterPath(clusterName, "ssh", "id_rsa"),
					meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
				ClusterSSH(metadata.Topology, metadata.User).
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
	return cmd
}
