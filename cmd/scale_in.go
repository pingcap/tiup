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

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiops/pkg/bindversion"
	"github.com/pingcap-incubator/tiops/pkg/cliutil"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/logger"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiops/pkg/task"
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
				if err := cliutil.PromptForConfirmOrAbortError(
					"This operation will delete the %s nodes in `%s` and all their data.\nDo you want to continue? [y/N]:",
					strings.Join(options.Nodes, ","),
					color.HiYellowString(clusterName)); err != nil {
					return err
				}
				log.Infof("Scale-in nodes...")
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

func scaleIn(clusterName string, options operator.Options) error {
	if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
		return errors.Errorf("cannot scale-in non-exists cluster %s", clusterName)
	}

	metadata, err := meta.ClusterMetadata(clusterName)
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
			t := task.NewBuilder()
			switch instance.ComponentName() {
			case meta.ComponentGrafana,
				meta.ComponentPrometheus:
				if instance.IsImported() {
					version := bindversion.ComponentVersion(instance.ComponentName(), metadata.Version)
					t.CopyComponent(instance.ComponentName(), version, instance.GetHost(), deployDir)
				}
			}
			t.InitConfig(clusterName,
				instance,
				metadata.User,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDir,
					Log:    logDir,
					Cache:  meta.ClusterPath(clusterName, "config"),
				},
			)
			regenConfigTasks = append(regenConfigTasks, t.Build())
		}
	}

	t := task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(clusterName, "ssh", "id_rsa"),
			meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User).
		ClusterOperate(metadata.Topology, operator.ScaleInOperation, options).
		UpdateMeta(clusterName, metadata, operator.AsyncNodes(metadata.Topology, options.Nodes, false)).
		Parallel(regenConfigTasks...).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return errors.Trace(err)
	}

	return nil
}
