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
	"path/filepath"
	"strings"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiops/pkg/logger"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newReloadCmd() *cobra.Command {
	var options operator.Options

	cmd := &cobra.Command{
		Use:   "reload <cluster-name>",
		Short: "Reload a TiDB cluster's config and restart if needed",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			if utils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return errors.Errorf("cannot start non-exists cluster %s", clusterName)
			}

			logger.EnableAuditLog()
			metadata, err := meta.ClusterMetadata(clusterName)
			if err != nil {
				return err
			}

			t, err := buildReloadTask(clusterName, metadata, options)
			if err != nil {
				return err
			}

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

	cmd.Flags().StringSliceVarP(&options.Roles, "role", "R", nil, "Only start specified roles")
	cmd.Flags().StringSliceVarP(&options.Nodes, "node", "N", nil, "Only start specified nodes")
	return cmd
}

func buildReloadTask(
	clusterName string,
	metadata *meta.ClusterMeta,
	options operator.Options,
) (task.Task, error) {

	var refreshConfigTasks []task.Task

	topo := metadata.Topology

	topo.IterInstance(func(inst meta.Instance) {
		deployDir := inst.DeployDir()
		if !strings.HasPrefix(deployDir, "/") {
			deployDir = filepath.Join("/home/", metadata.User, deployDir)
		}
		dataDir := inst.DataDir()
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join("/home/", metadata.User, dataDir)
		}
		logDir := inst.LogDir()
		if !strings.HasPrefix(logDir, "/") {
			logDir = filepath.Join("/home/", metadata.User, logDir)
		}
		// Refresh all configuration
		t := task.NewBuilder().
			UserSSH(inst.GetHost(), metadata.User).
			InitConfig(clusterName, inst, metadata.User, meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDir,
				Log:    logDir,
				Cache:  meta.ClusterPath(clusterName, "config"),
			}).
			Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	task := task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(clusterName, "ssh", "id_rsa"),
			meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User).
		Parallel(refreshConfigTasks...).
		ClusterOperate(metadata.Topology, operator.UpgradeOperation, options).
		Build()

	return task, nil
}
