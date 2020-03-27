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
	"fmt"
	"os"

	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/task"
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
				cmd.Help()
				return fmt.Errorf("cluster name not specified")
			}
			metadata, err := meta.ClusterMetadata(os.Args[1])
			if err != nil {
				return err
			}

			var shellTasks []task.Task
			for _, comp := range metadata.Topology.ComponentsByStartOrder() {
				for _, inst := range comp.Instances() {
					t := task.NewBuilder().
						UserSSH(inst.GetHost(), metadata.User).
						Shell(inst.GetHost(), opt.command, opt.sudo).
						Build()
					shellTasks = append(shellTasks, t)
				}
			}
			t := task.NewBuilder().
				Parallel(shellTasks...).
				Build()
			return t.Execute(task.NewContext())
		},
	}

	cmd.Flags().StringVar(&opt.command, "command", "ls", "the command run on cluster host")
	cmd.Flags().BoolVar(&opt.sudo, "sudo", false, "use root permissions (default false)")
	return cmd
}
