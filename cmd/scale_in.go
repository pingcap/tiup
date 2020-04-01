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
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/logger"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newScaleInCmd() *cobra.Command {
	var options operator.Options
	var skipConfirm bool
	cmd := &cobra.Command{
		Use:   "scale-in <cluster-name>",
		Short: "Scale in a TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			if !skipConfirm {
				promptMsg := fmt.Sprintf("This operation will delete the %s cluster nodes`%s` and delete those data, do you want to continue?\n[Y]es/[N]o:",
					strings.Join(options.Nodes, ","), color.HiYellowString(clusterName))
				if input, confirm := utils.Confirm(promptMsg); confirm {
					log.Infof("Scale-in nodes...")
				} else {
					return errors.Errorf("operation cancelled by user (input: %s)", input)
				}
			}

			logger.EnableAuditLog()
			return scaleIn(clusterName, options)
		},
	}

	cmd.Flags().StringSliceVarP(&options.Nodes, "node", "N", nil, "Specify the nodes")
	cmd.Flags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip the confirmation of destroying")
	_ = cmd.MarkFlagRequired("node")

	return cmd
}

func scaleIn(cluster string, options operator.Options) error {
	if tiuputils.IsNotExist(meta.ClusterPath(cluster, meta.MetaFileName)) {
		return errors.Errorf("cannot scale-in non-exists cluster %s", cluster)
	}

	metadata, err := meta.ClusterMetadata(cluster)
	if err != nil {
		return err
	}

	// Regenerate configuration
	var regenConfigTasks []task.Task
	deletedNodes := set.NewStringSet(options.Nodes...)
	for _, component := range metadata.Topology.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			if deletedNodes.Exist(instance.ID()) {
				continue
			}
			deployDir := instance.DeployDir()
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/", metadata.User, deployDir)
			}
			dataDir := instance.DataDir()
			if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join("/home/", metadata.User, dataDir)
			}
			logDir := instance.LogDir()
			if !strings.HasPrefix(logDir, "/") {
				logDir = filepath.Join("/home/", metadata.User, logDir)
			}
			t := task.NewBuilder().InitConfig(cluster,
				instance,
				metadata.User,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDir,
					Log:    logDir,
				},
			).Build()
			regenConfigTasks = append(regenConfigTasks, t)
		}
	}

	t := task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(cluster, "ssh", "id_rsa"),
			meta.ClusterPath(cluster, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User).
		ClusterOperate(metadata.Topology, operator.ScaleInOperation, options).
		UpdateMeta(cluster, metadata, operator.AsyncNodes(metadata.Topology, options.Nodes, false)).
		Parallel(regenConfigTasks...).
		Build()

	return t.Execute(task.NewContext())
}
