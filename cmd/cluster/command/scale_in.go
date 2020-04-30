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
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/clusterutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/logger"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	operator "github.com/pingcap-incubator/tiup-cluster/pkg/operation"
	"github.com/pingcap-incubator/tiup-cluster/pkg/task"
	"github.com/pingcap-incubator/tiup/pkg/set"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newScaleInCmd() *cobra.Command {
	var (
		options operator.Options
	)
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
	cmd.Flags().Int64Var(&options.Timeout, "transfer-timeout", 300, "Timeout in seconds when transferring PD and TiKV store leaders")
	cmd.Flags().BoolVar(&options.Force, "force", false, "Force just try stop and destroy instance before removing the instance from topo")

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
			deployDir := clusterutil.Abs(metadata.User, instance.DeployDir())
			// data dir would be empty for components which don't need it
			dataDir := clusterutil.Abs(metadata.User, instance.DataDir())
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(metadata.User, instance.LogDir())

			// Download and copy the latest component to remote if the cluster is imported from Ansible
			tb := task.NewBuilder()
			if instance.IsImported() {
				switch compName := instance.ComponentName(); compName {
				case meta.ComponentGrafana, meta.ComponentPrometheus, meta.ComponentAlertManager:
					version := meta.ComponentVersion(compName, metadata.Version)
					tb.Download(compName, version).CopyComponent(compName, version, instance.GetHost(), deployDir)
				}
			}

			t := tb.InitConfig(clusterName,
				metadata.Version,
				instance,
				metadata.User,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDir,
					Log:    logDir,
					Cache:  meta.ClusterPath(clusterName, "config"),
				},
			).Build()
			regenConfigTasks = append(regenConfigTasks, t)
		}
	}

	b := task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(clusterName, "ssh", "id_rsa"),
			meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User, sshTimeout)

	if !options.Force {
		b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, options).
			UpdateMeta(clusterName, metadata, operator.AsyncNodes(metadata.Topology, options.Nodes, false))
	} else {
		b.ClusterOperate(metadata.Topology, operator.ScaleInOperation, options).
			UpdateMeta(clusterName, metadata, options.Nodes)
	}

	t := b.Parallel(regenConfigTasks...).Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return errors.Trace(err)
	}

	log.Infof("Scaled cluster `%s` in successfully", clusterName)

	return nil
}
