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

	"github.com/pingcap-incubator/tiops/pkg/bindversion"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

type upgradeOptions struct {
	options operator.Options
}

func newUpgradeCmd() *cobra.Command {
	opt := upgradeOptions{}
	cmd := &cobra.Command{
		Use:   "upgrade <cluster-name> <version>",
		Short: "Upgrade a specified TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			auditConfig.enable = true
			return upgrade(args[0], args[1], opt)
		},
	}
	cmd.Flags().BoolVar(&opt.options.Force, "force", false, "Force upgrade won't transfer leader")
	return cmd
}

func versionCompare(curVersion, newVersion string) error {
	switch semver.Compare(curVersion, newVersion) {
	case -1:
		return nil
	case 1:
		if repository.Version(newVersion).IsNightly() {
			return nil
		}
		return errors.New(fmt.Sprintf("unsupport upgrade from %s to %s", curVersion, newVersion))
	default:
		return errors.Errorf("please specify a higher version than %s", curVersion)
	}
}

func upgrade(name, version string, opt upgradeOptions) error {
	if utils.IsNotExist(meta.ClusterPath(name, meta.MetaFileName)) {
		return errors.Errorf("cannot upgrade non-exists cluster %s", name)
	}

	metadata, err := meta.ClusterMetadata(name)
	if err != nil {
		return err
	}

	var (
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueComps = map[componentInfo]struct{}{}
	)

	if err := versionCompare(metadata.Version, version); err != nil {
		return err
	}

	for _, comp := range metadata.Topology.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			version := bindversion.ComponentVersion(inst.ComponentName(), version)
			if version == "" {
				return errors.Errorf("unsupported component: %v", inst.ComponentName())
			}
			compInfo := componentInfo{
				component: inst.ComponentName(),
				version:   version,
			}

			// Download component from repository
			if _, found := uniqueComps[compInfo]; !found {
				uniqueComps[compInfo] = struct{}{}
				t := task.NewBuilder().
					Download(inst.ComponentName(), version).
					Build()
				downloadCompTasks = append(downloadCompTasks, t)
			}

			deployDir := inst.DeployDir()
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/", metadata.User, deployDir)
			}
			// Deploy component
			t := task.NewBuilder().
				BackupComponent(inst.ComponentName(), metadata.Version, inst.GetHost(), deployDir).
				CopyComponent(inst.ComponentName(), version, inst.GetHost(), deployDir).
				Build()
			copyCompTasks = append(copyCompTasks, t)
		}
	}

	t := task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(name, "ssh", "id_rsa"),
			meta.ClusterPath(name, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User).
		Parallel(downloadCompTasks...).
		Parallel(copyCompTasks...).
		ClusterOperate(metadata.Topology, operator.UpgradeOperation, opt.options).
		Build()

	err = t.Execute(task.NewContext())
	if err != nil {
		return err
	}

	metadata.Version = version
	return meta.SaveClusterMeta(name, metadata)
}
