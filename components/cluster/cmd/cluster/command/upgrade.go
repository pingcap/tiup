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
	"fmt"
	"os"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/clusterutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/logger"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	operator "github.com/pingcap-incubator/tiup-cluster/pkg/operation"
	"github.com/pingcap-incubator/tiup-cluster/pkg/task"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/version"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade <cluster-name> <version>",
		Short: "Upgrade a specified TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			logger.EnableAuditLog()
			return upgrade(args[0], args[1], gOpt)
		},
	}
	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force upgrade won't transfer leader")
	cmd.Flags().Int64Var(&gOpt.APITimeout, "transfer-timeout", 300, "Timeout in seconds when transferring PD and TiKV store leaders")

	return cmd
}

func versionCompare(curVersion, newVersion string) error {
	// Can always upgrade to 'nightly' event the current version is 'nightly'
	if newVersion == version.NightlyVersion {
		return nil
	}

	switch semver.Compare(curVersion, newVersion) {
	case -1:
		return nil
	case 0, 1:
		return errors.Errorf("please specify a higher version than %s", curVersion)
	default:
		return errors.Errorf("unreachable")
	}
}

func upgrade(clusterName, clusterVersion string, opt operator.Options) error {
	if utils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
		return errors.Errorf("cannot upgrade non-exists cluster %s", clusterName)
	}

	metadata, err := meta.ClusterMetadata(clusterName)
	if err != nil {
		return err
	}

	var (
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueComps = map[string]struct{}{}
	)

	if err := versionCompare(metadata.Version, clusterVersion); err != nil {
		return err
	}

	hasImported := false
	for _, comp := range metadata.Topology.ComponentsByUpdateOrder() {
		for _, inst := range comp.Instances() {
			version := meta.ComponentVersion(inst.ComponentName(), clusterVersion)
			if version == "" {
				return errors.Errorf("unsupported component: %v", inst.ComponentName())
			}
			compInfo := componentInfo{
				component: inst.ComponentName(),
				version:   version,
			}

			// Download component from repository
			key := fmt.Sprintf("%s-%s-%s-%s", compInfo.component, compInfo.version, inst.OS(), inst.Arch())
			if _, found := uniqueComps[key]; !found {
				uniqueComps[key] = struct{}{}
				t := task.NewBuilder().
					Download(inst.ComponentName(), inst.OS(), inst.Arch(), version).
					Build()
				downloadCompTasks = append(downloadCompTasks, t)
			}

			deployDir := clusterutil.Abs(metadata.User, inst.DeployDir())
			// data dir would be empty for components which don't need it
			dataDirs := clusterutil.MultiDirAbs(metadata.User, inst.DataDir())
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(metadata.User, inst.LogDir())

			// Deploy component
			tb := task.NewBuilder()
			if inst.IsImported() {
				switch inst.ComponentName() {
				case meta.ComponentPrometheus, meta.ComponentGrafana, meta.ComponentAlertManager:
					tb.CopyComponent(inst.ComponentName(), inst.OS(), inst.Arch(), version, inst.GetHost(), deployDir)
				default:
					tb.BackupComponent(inst.ComponentName(), metadata.Version, inst.GetHost(), deployDir).
						CopyComponent(inst.ComponentName(), inst.OS(), inst.Arch(), version, inst.GetHost(), deployDir)
				}
				hasImported = true
			} else {
				tb.BackupComponent(inst.ComponentName(), metadata.Version, inst.GetHost(), deployDir).
					CopyComponent(inst.ComponentName(), inst.OS(), inst.Arch(), version, inst.GetHost(), deployDir)
			}
			tb.InitConfig(
				clusterName,
				clusterVersion,
				inst,
				metadata.User,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  meta.ClusterPath(clusterName, meta.TempConfigPath),
				},
			)
			copyCompTasks = append(copyCompTasks, tb.Build())
		}
	}

	// handle dir scheme changes
	if hasImported {
		if err := meta.HandleImportPathMigration(clusterName); err != nil {
			return err
		}
	}

	t := task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(clusterName, "ssh", "id_rsa"),
			meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User, gOpt.SSHTimeout).
		Parallel(downloadCompTasks...).
		Parallel(copyCompTasks...).
		ClusterOperate(metadata.Topology, operator.UpgradeOperation, opt).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return errors.Trace(err)
	}

	metadata.Version = clusterVersion
	if err := meta.SaveClusterMeta(clusterName, metadata); err != nil {
		return errors.Trace(err)
	}
	if err := os.RemoveAll(meta.ClusterPath(clusterName, "patch")); err != nil {
		return errors.Trace(err)
	}

	log.Infof("Upgraded cluster `%s` successfully", clusterName)

	return nil
}
