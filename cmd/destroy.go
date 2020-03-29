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
	"os"

	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy <cluster-name>",
		Short: "Destroy a specified cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			auditConfig.enable = true
			clusterName := args[0]
			metadata, err := meta.ClusterMetadata(clusterName)
			if err != nil {
				return err
			}
			t := task.NewBuilder().
				SSHKeySet(
					meta.ClusterPath(clusterName, "ssh", "id_rsa"),
					meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
				ClusterSSH(metadata.Topology, metadata.User).
				ClusterOperate(metadata.Topology, operator.StopOperation, operator.Options{}).
				ClusterOperate(metadata.Topology, operator.DestroyOperation, operator.Options{}).
				Build()

			if err := t.Execute(task.NewContext()); err != nil {
				return err
			}
			if err := os.RemoveAll(meta.ClusterPath(clusterName)); err != nil {
				return errors.Trace(err)
			}
			log.Infof("Destroy cluster `%s` successfully", clusterName)
			return nil
		},
	}
	return cmd
}
