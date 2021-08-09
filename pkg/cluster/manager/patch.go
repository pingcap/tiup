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

package manager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

// Patch the cluster.
func (m *Manager) Patch(name string, packagePath string, opt operator.Options, overwrite, offline, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	if !utils.IsExist(packagePath) {
		return perrs.Errorf("specified package(%s) not exists", packagePath)
	}

	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(
			fmt.Sprintf("Will patch the cluster %s with package path is %s, nodes: %s, roles: %s.\nDo you want to continue? [y/N]:",
				color.HiYellowString(name),
				color.HiYellowString(packagePath),
				color.HiRedString(strings.Join(opt.Nodes, ",")),
				color.HiRedString(strings.Join(opt.Roles, ",")),
			),
		); err != nil {
			return err
		}
	}

	insts, err := instancesToPatch(topo, opt)
	if err != nil {
		return err
	}
	if err := checkPackage(m.bindVersion, m.specManager, name, insts[0].ComponentName(), insts[0].OS(), insts[0].Arch(), packagePath); err != nil {
		return err
	}

	var replacePackageTasks []task.Task
	for _, inst := range insts {
		deployDir := spec.Abs(base.User, inst.DeployDir())
		tb := task.NewBuilder()
		tb.BackupComponent(inst.ComponentName(), base.Version, inst.GetHost(), deployDir).
			InstallPackage(packagePath, inst.GetHost(), deployDir)
		replacePackageTasks = append(replacePackageTasks, tb.Build())
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}
	b, err := m.sshTaskBuilder(name, topo, base.User, opt)
	if err != nil {
		return err
	}
	t := b.Parallel(false, replacePackageTasks...).
		Func("UpgradeCluster", func(ctx context.Context) error {
			if offline {
				return nil
			}
			return operator.Upgrade(ctx, topo, opt, tlsCfg)
		}).
		Build()

	if err := t.Execute(ctxt.New(context.Background(), opt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if overwrite {
		if err := overwritePatch(m.specManager, name, insts[0].ComponentName(), packagePath); err != nil {
			return err
		}
	}

	// mark instance as patched in meta
	topo.IterInstance(func(ins spec.Instance) {
		for _, pachedIns := range insts {
			if ins.ID() == pachedIns.ID() {
				ins.SetPatched(true)
				break
			}
		}
	})
	return m.specManager.SaveMeta(name, metadata)
}

func checkPackage(bindVersion spec.BindVersion, specManager *spec.SpecManager, name, comp, nodeOS, arch, packagePath string) error {
	metadata := specManager.NewMetadata()
	if err := specManager.Metadata(name, metadata); err != nil {
		return err
	}

	ver := bindVersion(comp, metadata.GetBaseMeta().Version)
	repo, err := clusterutil.NewRepository(nodeOS, arch)
	if err != nil {
		return err
	}
	entry, err := repo.ComponentBinEntry(comp, ver)
	if err != nil {
		return err
	}

	checksum, err := utils.Checksum(packagePath)
	if err != nil {
		return err
	}
	cacheDir := specManager.Path(name, "cache", comp+"-"+checksum[:7])
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return errors.Annotatef(err, "create cache directory %s", cacheDir)
	}
	if err := exec.Command("tar", "-xvf", packagePath, "-C", cacheDir).Run(); err != nil {
		return errors.Annotatef(err, "decompress %s", packagePath)
	}

	fi, err := os.Stat(path.Join(cacheDir, entry))
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Errorf("entry %s not found in package %s", entry, packagePath)
		}
		return errors.AddStack(err)
	}
	if !fi.Mode().IsRegular() {
		return errors.Errorf("entry %s in package %s is not a regular file", entry, packagePath)
	}
	if fi.Mode()&0500 != 0500 {
		return errors.Errorf("entry %s in package %s is not executable", entry, packagePath)
	}

	return nil
}

func overwritePatch(specManager *spec.SpecManager, name, comp, packagePath string) error {
	if err := os.MkdirAll(specManager.Path(name, spec.PatchDirName), 0755); err != nil {
		return err
	}

	checksum, err := utils.Checksum(packagePath)
	if err != nil {
		return err
	}

	tg := specManager.Path(name, spec.PatchDirName, comp+"-"+checksum[:7]+".tar.gz")
	if !utils.IsExist(tg) {
		if err := utils.Copy(packagePath, tg); err != nil {
			return err
		}
	}

	symlink := specManager.Path(name, spec.PatchDirName, comp+".tar.gz")
	if utils.IsSymExist(symlink) {
		os.Remove(symlink)
	}
	return os.Symlink(tg, symlink)
}

func instancesToPatch(topo spec.Topology, options operator.Options) ([]spec.Instance, error) {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := topo.ComponentsByStartOrder()
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
