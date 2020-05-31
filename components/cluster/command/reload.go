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
	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap-incubator/tiup/pkg/cluster/meta"
	operator "github.com/pingcap-incubator/tiup/pkg/cluster/operation"
	"github.com/pingcap-incubator/tiup/pkg/cluster/task"
	"github.com/pingcap-incubator/tiup/pkg/logger"
	log2 "github.com/pingcap-incubator/tiup/pkg/logger/log"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newReloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reload <cluster-name>",
		Short: "Reload a TiDB cluster's config and restart if needed",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
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

			t, err := buildReloadTask(clusterName, metadata, gOpt)
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

			log2.Infof("Reloaded cluster `%s` successfully", clusterName)

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only start specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only start specified nodes")
	cmd.Flags().Int64Var(&gOpt.APITimeout, "transfer-timeout", 300, "Timeout in seconds when transferring PD and TiKV store leaders")

	return cmd
}

func buildReloadTask(
	clusterName string,
	metadata *meta.ClusterMeta,
	options operator.Options,
) (task.Task, error) {

	var refreshConfigTasks []task.Task

	topo := metadata.Topology
	hasImported := false

	topo.IterInstance(func(inst meta.Instance) {
		deployDir := clusterutil.Abs(metadata.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(metadata.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(metadata.User, inst.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder().UserSSH(inst.GetHost(), inst.GetSSHPort(), metadata.User, gOpt.SSHTimeout)
		if inst.IsImported() {
			switch compName := inst.ComponentName(); compName {
			case meta.ComponentGrafana, meta.ComponentPrometheus, meta.ComponentAlertManager:
				version := meta.ComponentVersion(compName, metadata.Version)
				tb.Download(compName, inst.OS(), inst.Arch(), version).
					CopyComponent(compName, inst.OS(), inst.Arch(), version, inst.GetHost(), deployDir)
			}
			hasImported = true
		}

		// Refresh all configuration
		t := tb.InitConfig(clusterName,
			metadata.Version,
			inst, metadata.User,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  meta.ClusterPath(clusterName, meta.TempConfigPath),
			}).Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	// handle dir scheme changes
	if hasImported {
		if err := meta.HandleImportPathMigration(clusterName); err != nil {
			return task.NewBuilder().Build(), err
		}
	}

	t := task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(clusterName, "ssh", "id_rsa"),
			meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User, gOpt.SSHTimeout).
		Parallel(refreshConfigTasks...).
		ClusterOperate(metadata.Topology, operator.UpgradeOperation, options).
		Build()

	return t, nil
}

func validRoles(roles []string) error {
	for _, r := range roles {
		match := false
		for _, has := range meta.AllComponentNames() {
			if r == has {
				match = true
				break
			}
		}

		if !match {
			return errors.Errorf("not valid role: %s, should be one of: %v", r, meta.AllComponentNames())
		}
	}

	return nil
}
