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

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

// Upgrade the cluster.
func (m *Manager) Upgrade(name string, clusterVersion string, opt operator.Options, skipConfirm, offline bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// check locked
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// Adjust topo by new version
	if clusterTopo, ok := topo.(*spec.Specification); ok {
		clusterTopo.AdjustByVersion(clusterVersion)
	}

	var (
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueComps = map[string]struct{}{}
	)

	if err := versionCompare(base.Version, clusterVersion); err != nil {
		return err
	}

	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(
			"This operation will upgrade %s %s cluster %s to %s.\nDo you want to continue? [y/N]:",
			m.sysName,
			color.HiYellowString(base.Version),
			color.HiYellowString(name),
			color.HiYellowString(clusterVersion)); err != nil {
			return err
		}
		m.logger.Infof("Upgrading cluster...")
	}

	hasImported := false
	for _, comp := range topo.ComponentsByUpdateOrder() {
		for _, inst := range comp.Instances() {
			compName := inst.ComponentName()

			// ignore monitor agents for instances marked as ignore_exporter
			switch compName {
			case spec.ComponentNodeExporter,
				spec.ComponentBlackboxExporter:
				if inst.IgnoreMonitorAgent() {
					continue
				}
			}

			version := m.bindVersion(inst.ComponentName(), clusterVersion)

			// Download component from repository
			key := fmt.Sprintf("%s-%s-%s-%s", compName, version, inst.OS(), inst.Arch())
			if _, found := uniqueComps[key]; !found {
				uniqueComps[key] = struct{}{}
				t := task.NewBuilder(m.logger).
					Download(inst.ComponentName(), inst.OS(), inst.Arch(), version).
					Build()
				downloadCompTasks = append(downloadCompTasks, t)
			}

			deployDir := spec.Abs(base.User, inst.DeployDir())
			// data dir would be empty for components which don't need it
			dataDirs := spec.MultiDirAbs(base.User, inst.DataDir())
			// log dir will always be with values, but might not used by the component
			logDir := spec.Abs(base.User, inst.LogDir())

			// Deploy component
			tb := task.NewBuilder(m.logger)

			// for some component, dataDirs might need to be created due to upgrade
			// eg: TiCDC support DataDir since v4.0.13
			tb = tb.Mkdir(topo.BaseTopo().GlobalOptions.User, inst.GetHost(), dataDirs...)

			if inst.IsImported() {
				switch inst.ComponentName() {
				case spec.ComponentPrometheus, spec.ComponentGrafana, spec.ComponentAlertmanager:
					tb.CopyComponent(
						inst.ComponentName(),
						inst.OS(),
						inst.Arch(),
						version,
						"", // use default srcPath
						inst.GetHost(),
						deployDir,
					)
				}
				hasImported = true
			}

			// backup files of the old version
			tb = tb.BackupComponent(inst.ComponentName(), base.Version, inst.GetHost(), deployDir)

			if deployerInstance, ok := inst.(DeployerInstance); ok {
				deployerInstance.Deploy(tb, "", deployDir, version, name, clusterVersion)
			} else {
				// copy dependency component if needed
				switch inst.ComponentName() {
				case spec.ComponentTiSpark:
					env := environment.GlobalEnv()
					sparkVer, _, err := env.V1Repository().WithOptions(repository.Options{
						GOOS:   inst.OS(),
						GOARCH: inst.Arch(),
					}).LatestStableVersion(spec.ComponentSpark, false)
					if err != nil {
						return err
					}
					tb = tb.DeploySpark(inst, sparkVer.String(), "" /* default srcPath */, deployDir)
				default:
					tb = tb.CopyComponent(
						inst.ComponentName(),
						inst.OS(),
						inst.Arch(),
						version,
						"", // use default srcPath
						inst.GetHost(),
						deployDir,
					)
				}
			}

			tb.InitConfig(
				name,
				clusterVersion,
				m.specManager,
				inst,
				base.User,
				opt.IgnoreConfigCheck,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  m.specManager.Path(name, spec.TempConfigPath),
				},
			)
			copyCompTasks = append(copyCompTasks, tb.Build())
		}
	}

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(name); err != nil {
			return err
		}
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}
	b, err := m.sshTaskBuilder(name, topo, base.User, opt)
	if err != nil {
		return err
	}
	t := b.
		Parallel(false, downloadCompTasks...).
		Parallel(opt.Force, copyCompTasks...).
		Func("UpgradeCluster", func(ctx context.Context) error {
			if offline {
				return nil
			}
			return operator.Upgrade(ctx, topo, opt, tlsCfg)
		}).
		Build()

	ctx := ctxt.New(
		context.Background(),
		opt.Concurrency,
		m.logger,
	)
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	// clear patched packages and tags
	if err := os.RemoveAll(m.specManager.Path(name, "patch")); err != nil {
		return perrs.Trace(err)
	}
	topo.IterInstance(func(ins spec.Instance) {
		if ins.IsPatched() {
			ins.SetPatched(false)
		}
	})

	metadata.SetVersion(clusterVersion)

	if err := m.specManager.SaveMeta(name, metadata); err != nil {
		return err
	}

	m.logger.Infof("Upgraded cluster `%s` successfully", name)

	return nil
}

func versionCompare(curVersion, newVersion string) error {
	// Can always upgrade to 'nightly' event the current version is 'nightly'
	if newVersion == utils.NightlyVersionAlias {
		return nil
	}

	switch semver.Compare(curVersion, newVersion) {
	case -1:
		return nil
	case 0, 1:
		return perrs.Errorf("please specify a higher version than %s", curVersion)
	default:
		return perrs.Errorf("unreachable")
	}
}
