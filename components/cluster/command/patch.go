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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newPatchCmd() *cobra.Command {
	var (
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "patch <cluster-name> <package-path>",
		Short: "Replace the remote package with a specified package and restart the service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}

			if len(gOpt.Nodes) == 0 && len(gOpt.Roles) == 0 {
				return perrs.New("the flag -R or -N must be specified at least one")
			}
			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))
			return patch(args[0], args[1], gOpt, overwrite)
		},
	}

	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Use this package in the future scale-out operations")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Specify the nodes")
	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Specify the role")
	cmd.Flags().Int64Var(&gOpt.APITimeout, "transfer-timeout", 300, "Timeout in seconds when transferring PD and TiKV store leaders")
	return cmd
}

func patch(clusterName, packagePath string, options operator.Options, overwrite bool) error {
	if tiuputils.IsNotExist(spec.ClusterPath(clusterName, spec.MetaFileName)) {
		return perrs.Errorf("cannot patch non-exists cluster %s", clusterName)
	}

	if exist := tiuputils.IsExist(packagePath); !exist {
		return perrs.New("specified package not exists")
	}

	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	insts, err := instancesToPatch(metadata, options)
	if err != nil {
		return err
	}
	if err := checkPackage(clusterName, insts[0].ComponentName(), insts[0].OS(), insts[0].Arch(), packagePath); err != nil {
		return err
	}

	var replacePackageTasks []task.Task
	for _, inst := range insts {
		deployDir := clusterutil.Abs(metadata.User, inst.DeployDir())
		tb := task.NewBuilder()
		tb.BackupComponent(inst.ComponentName(), metadata.Version, inst.GetHost(), deployDir).
			InstallPackage(packagePath, inst.GetHost(), deployDir)
		replacePackageTasks = append(replacePackageTasks, tb.Build())
	}

	t := task.NewBuilder().
		SSHKeySet(
			spec.ClusterPath(clusterName, "ssh", "id_rsa"),
			spec.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.Topology, metadata.User, gOpt.SSHTimeout).
		Parallel(replacePackageTasks...).
		ClusterOperate(metadata.Topology, operator.UpgradeOperation, options).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if overwrite {
		if err := overwritePatch(clusterName, insts[0].ComponentName(), packagePath); err != nil {
			return err
		}
	}

	return nil
}

func instancesToPatch(metadata *spec.ClusterMeta, options operator.Options) ([]spec.Instance, error) {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := metadata.Topology.ComponentsByStartOrder()
	components = operator.FilterComponent(components, roleFilter)

	instances := []spec.Instance{}
	comps := []string{}
	for _, com := range components {
		insts := operator.FilterInstance(com.Instances(), nodeFilter)
		if len(insts) > 0 {
			comps = append(comps, com.Name())
		}
		instances = append(instances, insts...)
	}
	if len(comps) > 1 {
		return nil, fmt.Errorf("can't patch more than one component at once: %v", comps)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instance found on specifid role(%v) and nodes(%v)", options.Roles, options.Nodes)
	}

	return instances, nil
}

func checkPackage(clusterName, comp, nodeOS, arch, packagePath string) error {
	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	ver := spec.ComponentVersion(comp, metadata.Version)
	repo, err := clusterutil.NewRepository(nodeOS, arch)
	if err != nil {
		return err
	}
	entry, err := repo.ComponentBinEntry(comp, ver)
	if err != nil {
		return err
	}

	checksum, err := tiuputils.Checksum(packagePath)
	if err != nil {
		return err
	}
	cacheDir := spec.ClusterPath(clusterName, "cache", comp+"-"+checksum[:7])
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	if err := exec.Command("tar", "-xvf", packagePath, "-C", cacheDir).Run(); err != nil {
		return err
	}

	if exists := tiuputils.IsExist(path.Join(cacheDir, entry)); !exists {
		return fmt.Errorf("entry %s not found in package %s", entry, packagePath)
	}

	return nil
}

func overwritePatch(clusterName, comp, packagePath string) error {
	if err := os.MkdirAll(spec.ClusterPath(clusterName, spec.PatchDirName), 0755); err != nil {
		return err
	}

	checksum, err := tiuputils.Checksum(packagePath)
	if err != nil {
		return err
	}

	tg := spec.ClusterPath(clusterName, spec.PatchDirName, comp+"-"+checksum[:7]+".tar.gz")
	if utils.IsExist(tg) {
		os.Remove(tg)
	}
	if err := tiuputils.CopyFile(packagePath, tg); err != nil {
		return err
	}

	symlink := spec.ClusterPath(clusterName, spec.PatchDirName, comp+".tar.gz")
	if utils.IsExist(symlink) {
		os.Remove(symlink)
	}
	return os.Symlink(tg, symlink)
}
