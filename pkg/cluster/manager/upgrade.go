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
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

// Upgrade the cluster.
func (m *Manager) Upgrade(name string, clusterVersion string, componentVersions map[string]string, opt operator.Options, skipConfirm, offline, ignoreVersionCheck bool) error {
	if !skipConfirm && strings.ToLower(opt.DisplayMode) != "json" {
		for _, v := range componentVersions {
			if v != "" {
				m.logger.Warnf(color.YellowString("tiup-cluster does not provide compatibility guarantees or version checks for different component versions. Please be aware of the risks or use it with the assistance of PingCAP support."))
				err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
				if err != nil {
					return err
				}
				break
			}
		}
	}
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
		if !ignoreVersionCheck {
			return err
		}
		m.logger.Warnf(color.RedString("There is no guarantee that the cluster can be downgraded. Be careful before you continue."))
	}

	compVersionMsg := ""
	restartComponents := []string{}
	components := topo.ComponentsByUpdateOrder(base.Version)
	for _, comp := range components {
		// if component version is not specified, use the cluster version or latest("")
		oldver := comp.CalculateVersion(base.Version)
		version := componentVersions[comp.Name()]
		if version != "" {
			comp.SetVersion(version)
		}
		calver := comp.CalculateVersion(clusterVersion)
		if comp.Name() != spec.ComponentTiProxy || calver != oldver {
			restartComponents = append(restartComponents, comp.Name(), comp.Role())
			if len(comp.Instances()) > 0 {
				compVersionMsg += fmt.Sprintf("\nwill upgrade and restart component \"%19s\" to \"%s\",", comp.Name(), calver)
			}
		}
	}
	components = operator.FilterComponent(components, set.NewStringSet(restartComponents...))

	monitoredOptions := topo.GetMonitoredOptions()
	if monitoredOptions != nil {
		if componentVersions[spec.ComponentBlackboxExporter] != "" {
			monitoredOptions.BlackboxExporterVersion = componentVersions[spec.ComponentBlackboxExporter]
		}
		if componentVersions[spec.ComponentNodeExporter] != "" {
			monitoredOptions.NodeExporterVersion = componentVersions[spec.ComponentNodeExporter]
		}
		compVersionMsg += fmt.Sprintf("\nwill upgrade component %19s to \"%s\",", "\"node-exporter\"", monitoredOptions.NodeExporterVersion)
		compVersionMsg += fmt.Sprintf("\nwill upgrade component %19s to \"%s\".", "\"blackbox-exporter\"", monitoredOptions.BlackboxExporterVersion)
	}

	m.logger.Warnf(`%s
This operation will upgrade %s %s cluster %s to %s:%s`,
		color.YellowString("Before the upgrade, it is recommended to read the upgrade guide at https://docs.pingcap.com/tidb/stable/upgrade-tidb-using-tiup and finish the preparation steps."),
		m.sysName,
		color.HiYellowString(base.Version),
		color.HiYellowString(name),
		color.HiYellowString(clusterVersion),
		compVersionMsg)
	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(`Do you want to continue? [y/N]:`); err != nil {
			return err
		}
		m.logger.Infof("Upgrading cluster...")
	}

	hasImported := false
	for _, comp := range components {
		version := comp.CalculateVersion(clusterVersion)

		for _, inst := range comp.Instances() {
			// Download component from repository
			key := fmt.Sprintf("%s-%s-%s-%s", inst.ComponentSource(), version, inst.OS(), inst.Arch())
			if _, found := uniqueComps[key]; !found {
				uniqueComps[key] = struct{}{}
				t := task.NewBuilder(m.logger).
					Download(inst.ComponentSource(), inst.OS(), inst.Arch(), version).
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
			tb = tb.Mkdir(topo.BaseTopo().GlobalOptions.User, inst.GetManageHost(), topo.BaseTopo().GlobalOptions.SystemdMode != spec.UserMode, dataDirs...)

			if inst.IsImported() {
				switch inst.ComponentName() {
				case spec.ComponentPrometheus, spec.ComponentGrafana, spec.ComponentAlertmanager:
					tb.CopyComponent(
						inst.ComponentSource(),
						inst.OS(),
						inst.Arch(),
						version,
						"", // use default srcPath
						inst.GetManageHost(),
						deployDir,
					)
				}
				hasImported = true
			}

			// backup files of the old version
			tb = tb.BackupComponent(inst.ComponentSource(), base.Version, inst.GetManageHost(), deployDir)

			// this interface is not used
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
						inst.ComponentSource(),
						inst.OS(),
						inst.Arch(),
						version,
						"", // use default srcPath
						inst.GetManageHost(),
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

	var sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	if opt.SSHType != executor.SSHTypeNone {
		var err error
		if len(opt.SSHProxyHost) != 0 {
			if sshProxyProps, err = tui.ReadIdentityFileOrPassword(opt.SSHProxyIdentity, opt.SSHProxyUsePassword); err != nil {
				return err
			}
		}
	}

	uniqueHosts, noAgentHosts := getMonitorHosts(topo)
	// Deploy monitor relevant components to remote
	dlTasks, dpTasks, err := buildMonitoredDeployTask(
		m,
		uniqueHosts,
		noAgentHosts,
		topo.BaseTopo().GlobalOptions,
		monitoredOptions,
		opt,
		sshProxyProps,
	)
	if err != nil {
		return err
	}

	monitorConfigTasks := buildInitMonitoredConfigTasks(
		m.specManager,
		name,
		uniqueHosts,
		noAgentHosts,
		*topo.BaseTopo().GlobalOptions,
		monitoredOptions,
		m.logger,
		opt.SSHTimeout,
		opt.OptTimeout,
		opt,
		sshProxyProps,
	)

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(name); err != nil {
			return err
		}
	}
	ctx := ctxt.New(
		context.Background(),
		opt.Concurrency,
		m.logger,
	)
	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	// make sure the cluster is stopped
	if offline && !opt.Force {
		running := false
		topo.IterInstance(func(ins spec.Instance) {
			if !running {
				status := ins.Status(ctx, time.Duration(opt.APITimeout), tlsCfg, topo.BaseTopo().MasterList...)
				if strings.HasPrefix(status, "Up") || strings.HasPrefix(status, "Healthy") {
					running = true
				}
			}
		}, opt.Concurrency)

		if running {
			return perrs.Errorf("cluster is running and cannot be upgraded offline")
		}
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, opt)
	if err != nil {
		return err
	}
	t := b.
		Parallel(false, downloadCompTasks...).
		ParallelStep("download monitored", false, dlTasks...).
		Parallel(opt.Force, copyCompTasks...).
		ParallelStep("deploy monitored", false, dpTasks...).
		ParallelStep("refresh monitored config", false, monitorConfigTasks...).
		Func("UpgradeCluster", func(ctx context.Context) error {
			if offline {
				return nil
			}
			nopt := opt
			nopt.Roles = restartComponents
			return operator.Upgrade(ctx, topo, nopt, tlsCfg, base.Version, clusterVersion)
		}).
		Build()

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
	case -1, 0:
		return nil
	case 1:
		return perrs.Errorf("please specify a higher or equle version than %s", curVersion)
	default:
		return perrs.Errorf("unreachable")
	}
}
