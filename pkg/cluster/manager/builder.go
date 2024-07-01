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
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/environment"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

// buildReloadPromTasks reloads Prometheus and Grafana configuration
func buildReloadPromAndGrafanaTasks(
	topo spec.Topology,
	logger *logprinter.Logger,
	gOpt operator.Options,
	nodes ...string,
) []*task.StepDisplay {
	var instances []spec.Instance
	// get promtheus and grafana instance list
	monitor := spec.FindComponent(topo, spec.ComponentPrometheus)
	grafanas := spec.FindComponent(topo, spec.ComponentGrafana)

	instances = append(instances, monitor.Instances()...)
	instances = append(instances, grafanas.Instances()...)

	if len(instances) == 0 {
		return nil
	}
	var tasks []*task.StepDisplay
	deletedNodes := set.NewStringSet(nodes...)
	systemdMode := topo.BaseTopo().GlobalOptions.SystemdMode
	for _, inst := range instances {
		if deletedNodes.Exist(inst.ID()) {
			continue
		}

		t := task.NewBuilder(logger)
		if inst.ComponentName() == spec.ComponentPrometheus {
			// reload Prometheus
			t = t.SystemCtl(inst.GetManageHost(), inst.ServiceName(), "reload", true, true, string(systemdMode))
		} else {
			// restart grafana
			t = t.SystemCtl(inst.GetManageHost(), inst.ServiceName(), "restart", true, false, string(systemdMode))
		}

		tasks = append(tasks, t.BuildAsStep(fmt.Sprintf("  - Reload %s -> %s", inst.ComponentName(), inst.ID())))
	}
	return tasks
}

func buildScaleOutTask(
	m *Manager,
	name string,
	metadata spec.Metadata,
	mergedTopo spec.Topology,
	opt DeployOptions,
	s, p *tui.SSHConnectionProps,
	newPart spec.Topology,
	patchedComponents set.StringSet,
	gOpt operator.Options,
	afterDeploy func(b *task.Builder, newPart spec.Topology, gOpt operator.Options),
	final func(b *task.Builder, name string, meta spec.Metadata, gOpt operator.Options),
) (task.Task, error) {
	var (
		envInitTasks      []*task.StepDisplay // tasks which are used to initialize environment
		downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	specManager := m.specManager

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return nil, err
	}

	var sudo bool
	systemdMode := topo.BaseTopo().GlobalOptions.SystemdMode
	if systemdMode == spec.UserMode {
		sudo = false
	} else {
		sudo = true
	}

	// Initialize the environments
	initializedHosts := set.NewStringSet()
	metadata.GetTopology().IterInstance(func(instance spec.Instance) {
		initializedHosts.Insert(instance.GetManageHost())
	})
	// uninitializedHosts are hosts which haven't been initialized yet
	uninitializedHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	newPart.IterInstance(func(instance spec.Instance) {
		host := instance.GetManageHost()
		if initializedHosts.Exist(host) {
			return
		}
		if _, found := uninitializedHosts[host]; found {
			return
		}

		uninitializedHosts[host] = hostInfo{
			ssh:  instance.GetSSHPort(),
			os:   instance.OS(),
			arch: instance.Arch(),
		}

		var dirs []string
		globalOptions := metadata.GetTopology().BaseTopo().GlobalOptions
		for _, dir := range []string{globalOptions.DeployDir, globalOptions.DataDir, globalOptions.LogDir} {
			for _, dirname := range strings.Split(dir, ",") {
				if dirname == "" {
					continue
				}
				dirs = append(dirs, spec.Abs(globalOptions.User, dirname))
			}
		}
		if systemdMode == spec.UserMode {
			dirs = append(dirs, spec.Abs(globalOptions.User, ".config/systemd/user"))
		}
		t := task.NewBuilder(m.logger).
			RootSSH(
				instance.GetManageHost(),
				instance.GetSSHPort(),
				opt.User,
				s.Password,
				s.IdentityFile,
				s.IdentityFilePassphrase,
				gOpt.SSHTimeout,
				gOpt.OptTimeout,
				gOpt.SSHProxyHost,
				gOpt.SSHProxyPort,
				gOpt.SSHProxyUser,
				p.Password,
				p.IdentityFile,
				p.IdentityFilePassphrase,
				gOpt.SSHProxyTimeout,
				gOpt.SSHType,
				globalOptions.SSHType,
				opt.User != "root" && systemdMode != spec.UserMode,
			).
			EnvInit(instance.GetManageHost(), base.User, base.Group, opt.SkipCreateUser || globalOptions.User == opt.User, sudo).
			Mkdir(globalOptions.User, instance.GetManageHost(), sudo, dirs...).
			BuildAsStep(fmt.Sprintf("  - Initialized host %s ", host))
		envInitTasks = append(envInitTasks, t)
	})

	// Download missing component
	downloadCompTasks = buildDownloadCompTasks(
		base.Version,
		newPart,
		m.logger,
		gOpt,
	)

	sshType := topo.BaseTopo().GlobalOptions.SSHType

	var iterErr error
	// Deploy the new topology and refresh the configuration
	newPart.IterInstance(func(inst spec.Instance) {
		version := inst.CalculateVersion(base.Version)
		deployDir := spec.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := spec.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := spec.Abs(base.User, inst.LogDir())

		deployDirs := []string{
			deployDir,
			filepath.Join(deployDir, "bin"),
			filepath.Join(deployDir, "conf"),
			filepath.Join(deployDir, "scripts"),
		}
		// Deploy component
		tb := task.NewSimpleUerSSH(m.logger, inst.GetManageHost(), inst.GetSSHPort(), base.User, gOpt, p, sshType).
			Mkdir(base.User, inst.GetManageHost(), sudo, deployDirs...).
			Mkdir(base.User, inst.GetManageHost(), sudo, dataDirs...).
			Mkdir(base.User, inst.GetManageHost(), sudo, logDir)

		srcPath := ""
		if patchedComponents.Exist(inst.ComponentName()) {
			srcPath = specManager.Path(name, spec.PatchDirName, inst.ComponentName()+".tar.gz")
		}

		if deployerInstance, ok := inst.(DeployerInstance); ok {
			deployerInstance.Deploy(tb, srcPath, deployDir, version, name, version)
		} else {
			// copy dependency component if needed
			switch inst.ComponentName() {
			case spec.ComponentTiSpark:
				env := environment.GlobalEnv()
				var sparkVer utils.Version
				if sparkVer, _, iterErr = env.V1Repository().LatestStableVersion(spec.ComponentSpark, false); iterErr != nil {
					return
				}
				tb = tb.DeploySpark(inst, sparkVer.String(), srcPath, deployDir)
			default:
				tb.CopyComponent(
					inst.ComponentSource(),
					inst.OS(),
					inst.Arch(),
					inst.CalculateVersion(version),
					srcPath,
					inst.GetManageHost(),
					deployDir,
				)
			}
		}

		deployCompTasks = append(deployCompTasks, tb.BuildAsStep(fmt.Sprintf("  - Deploy instance %s -> %s", inst.ComponentName(), inst.ID())))
	})

	if iterErr != nil {
		return nil, iterErr
	}

	// Download and copy the latest component to remote if the cluster is imported from Ansible
	mergedTopo.IterInstance(func(inst spec.Instance) {
		if inst.IsImported() {
			deployDir := spec.Abs(base.User, inst.DeployDir())
			// data dir would be empty for components which don't need it
			// Download and copy the latest component to remote if the cluster is imported from Ansible
			tb := task.NewBuilder(m.logger)
			version := inst.CalculateVersion(base.Version)
			switch compName := inst.ComponentName(); compName {
			case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertmanager:
				tb.Download(compName, inst.OS(), inst.Arch(), version).
					CopyComponent(compName, inst.OS(), inst.Arch(), version, "", inst.GetManageHost(), deployDir)
			}
			deployCompTasks = append(deployCompTasks, tb.BuildAsStep(fmt.Sprintf("  - Deploy instance %s -> %s", inst.ComponentName(), inst.ID())))
		}
	})

	// init scale out config
	scaleOutConfigTasks := buildScaleConfigTasks(m, name, topo, newPart, base, gOpt, p)

	certificateTasks, err := buildCertificateTasks(m, name, newPart, base, gOpt, p)
	if err != nil {
		return nil, err
	}
	sessionCertTasks, err := buildSessionCertTasks(m, name, topo, newPart, base, gOpt, p)
	if err != nil {
		return nil, err
	}
	certificateTasks = append(certificateTasks, sessionCertTasks...)

	// always ignore config check result in scale out
	gOpt.IgnoreConfigCheck = true
	refreshConfigTasks, hasImported := buildInitConfigTasks(m, name, mergedTopo, base, gOpt, nil)
	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(name); err != nil {
			return task.NewBuilder(m.logger).Build(), err
		}
	}

	_, noAgentHosts := getMonitorHosts(mergedTopo)

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks, err := buildMonitoredDeployTask(
		m,
		uninitializedHosts,
		noAgentHosts,
		topo.BaseTopo().GlobalOptions,
		topo.BaseTopo().MonitoredOptions,
		gOpt,
		p,
	)
	if err != nil {
		return nil, err
	}

	downloadCompTasks = append(downloadCompTasks, dlTasks...)
	deployCompTasks = append(deployCompTasks, dpTasks...)

	// monitor config
	monitorConfigTasks := buildInitMonitoredConfigTasks(
		m.specManager,
		name,
		uninitializedHosts,
		noAgentHosts,
		*topo.BaseTopo().GlobalOptions,
		topo.GetMonitoredOptions(),
		m.logger,
		gOpt.SSHTimeout,
		gOpt.OptTimeout,
		gOpt,
		p,
	)

	// monitor tls file
	moniterCertificateTasks, err := buildMonitoredCertificateTasks(
		m,
		name,
		uninitializedHosts,
		noAgentHosts,
		topo.BaseTopo().GlobalOptions,
		topo.GetMonitoredOptions(),
		gOpt,
		p,
	)
	if err != nil {
		return nil, err
	}
	certificateTasks = append(certificateTasks, moniterCertificateTasks...)

	builder, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return nil, err
	}

	// stage2 just start and init config
	if !opt.Stage2 {
		builder.
			ParallelStep("+ Download TiDB components", gOpt.Force, downloadCompTasks...).
			ParallelStep("+ Initialize target host environments", gOpt.Force, envInitTasks...).
			ParallelStep("+ Deploy TiDB instance", gOpt.Force, deployCompTasks...).
			ParallelStep("+ Copy certificate to remote host", gOpt.Force, certificateTasks...).
			ParallelStep("+ Generate scale-out config", gOpt.Force, scaleOutConfigTasks...).
			ParallelStep("+ Init monitor config", gOpt.Force, monitorConfigTasks...)
	}

	if afterDeploy != nil {
		afterDeploy(builder, newPart, gOpt)
	}

	builder.Func("Save meta", func(_ context.Context) error {
		metadata.SetTopology(mergedTopo)
		return m.specManager.SaveMeta(name, metadata)
	})

	// don't start the new instance
	if opt.Stage1 {
		// save scale out file lock
		builder.Func("Create scale-out file lock", func(_ context.Context) error {
			return m.specManager.NewScaleOutLock(name, newPart)
		})
	} else {
		builder.Func("Start new instances", func(ctx context.Context) error {
			return operator.Start(ctx,
				newPart,
				operator.Options{
					OptTimeout: gOpt.OptTimeout,
					Operation:  operator.ScaleOutOperation,
				},
				false, /* restoreLeader */
				tlsCfg,
			)
		}).
			ParallelStep("+ Refresh components conifgs", gOpt.Force, refreshConfigTasks...).
			ParallelStep("+ Reload prometheus and grafana", gOpt.Force,
				buildReloadPromAndGrafanaTasks(metadata.GetTopology(), m.logger, gOpt)...)
	}

	// remove scale-out file lock
	if opt.Stage2 {
		builder.Func("Release Scale-Out File Lock", func(ctx context.Context) error {
			return m.specManager.ReleaseScaleOutLock(name)
		})
	}

	if final != nil {
		final(builder, name, metadata, gOpt)
	}
	return builder.Build(), nil
}

// buildScaleConfigTasks  generates certificate for instance and transfers it to the server
func buildScaleConfigTasks(
	m *Manager,
	name string,
	topo spec.Topology,
	newPart spec.Topology,
	base *spec.BaseMeta,
	gOpt operator.Options,
	p *tui.SSHConnectionProps) []*task.StepDisplay {
	var (
		scaleConfigTasks []*task.StepDisplay // tasks which are used to copy certificate to remote host
	)

	// copy certificate to remote host
	newPart.IterInstance(func(inst spec.Instance) {
		deployDir := spec.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := spec.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := spec.Abs(base.User, inst.LogDir())

		t := task.NewSimpleUerSSH(m.logger, inst.GetManageHost(), inst.GetSSHPort(), base.User, gOpt, p, topo.BaseTopo().GlobalOptions.SSHType).
			ScaleConfig(
				name,
				base.Version,
				m.specManager,
				topo,
				inst,
				base.User,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
				},
			).BuildAsStep(fmt.Sprintf("  - Generate scale-out config %s -> %s", inst.ComponentName(), inst.ID()))
		scaleConfigTasks = append(scaleConfigTasks, t)
	})

	return scaleConfigTasks
}

type hostInfo struct {
	ssh  int    // ssh port of host
	os   string // operating system
	arch string // cpu architecture
	// vendor string
}

func buildMonitoredDeployTask(
	m *Manager,
	uniqueHosts map[string]hostInfo, // host -> ssh-port, os, arch
	noAgentHosts set.StringSet, // hosts that do not deploy monitor agents
	globalOptions *spec.GlobalOptions,
	monitoredOptions *spec.MonitoredOptions,
	gOpt operator.Options,
	p *tui.SSHConnectionProps,
) (downloadCompTasks []*task.StepDisplay, deployCompTasks []*task.StepDisplay, err error) {
	if monitoredOptions == nil {
		return
	}

	uniqueCompOSArch := set.NewStringSet()
	// monitoring agents
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		version := monitoredOptions.NodeExporterVersion
		if comp == spec.ComponentBlackboxExporter {
			version = monitoredOptions.BlackboxExporterVersion
		}
		for host, info := range uniqueHosts {
			// skip deploying monitoring agents if the instance is marked so
			if noAgentHosts.Exist(host) {
				continue
			}

			// populate unique comp-os-arch set
			key := fmt.Sprintf("%s-%s-%s", comp, info.os, info.arch)
			if found := uniqueCompOSArch.Exist(key); !found {
				uniqueCompOSArch.Insert(key)
				downloadCompTasks = append(downloadCompTasks, task.NewBuilder(m.logger).
					Download(comp, info.os, info.arch, version).
					BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)", comp, version, info.os, info.arch)))
			}

			deployDir := spec.Abs(globalOptions.User, monitoredOptions.DeployDir)
			// data dir would be empty for components which don't need it
			dataDir := monitoredOptions.DataDir
			// the default data_dir is relative to deploy_dir
			if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join(deployDir, dataDir)
			}
			// log dir will always be with values, but might not used by the component
			logDir := spec.Abs(globalOptions.User, monitoredOptions.LogDir)

			deployDirs := []string{
				deployDir,
				dataDir,
				logDir,
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts"),
			}

			// Deploy component
			tb := task.NewSimpleUerSSH(m.logger, host, info.ssh, globalOptions.User, gOpt, p, globalOptions.SSHType).
				Mkdir(globalOptions.User, host, globalOptions.SystemdMode != spec.UserMode, deployDirs...).
				CopyComponent(
					comp,
					info.os,
					info.arch,
					version,
					"",
					host,
					deployDir,
				)
			deployCompTasks = append(deployCompTasks, tb.BuildAsStep(fmt.Sprintf("  - Deploy %s -> %s", comp, host)))
		}
	}
	return
}

// buildMonitoredCertificateTasks  generates certificate for instance and transfers it to the server
func buildMonitoredCertificateTasks(
	m *Manager,
	name string,
	uniqueHosts map[string]hostInfo, // host -> ssh-port, os, arch
	noAgentHosts set.StringSet, // hosts that do not deploy monitor agents
	globalOptions *spec.GlobalOptions,
	monitoredOptions *spec.MonitoredOptions,
	gOpt operator.Options,
	p *tui.SSHConnectionProps,
) ([]*task.StepDisplay, error) {
	var certificateTasks []*task.StepDisplay

	if monitoredOptions == nil {
		return certificateTasks, nil
	}

	if globalOptions.TLSEnabled {
		// monitoring agents
		for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
			for host, info := range uniqueHosts {
				// skip deploying monitoring agents if the instance is marked so
				if noAgentHosts.Exist(host) {
					continue
				}

				deployDir := spec.Abs(globalOptions.User, monitoredOptions.DeployDir)
				tlsDir := filepath.Join(deployDir, spec.TLSCertKeyDir)

				// Deploy component
				tb := task.NewSimpleUerSSH(m.logger, host, info.ssh, globalOptions.User, gOpt, p, globalOptions.SSHType).
					Mkdir(globalOptions.User, host, globalOptions.SystemdMode != spec.UserMode, tlsDir)

				if comp == spec.ComponentBlackboxExporter {
					ca, innerr := crypto.ReadCA(
						name,
						m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSCACert),
						m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSCAKey),
					)
					if innerr != nil {
						return certificateTasks, innerr
					}
					tb = tb.TLSCert(
						host,
						spec.ComponentBlackboxExporter,
						spec.ComponentBlackboxExporter,
						monitoredOptions.BlackboxExporterPort,
						ca,
						meta.DirPaths{
							Deploy: deployDir,
							Cache:  m.specManager.Path(name, spec.TempConfigPath),
						})
				}

				certificateTasks = append(certificateTasks, tb.BuildAsStep(fmt.Sprintf("  - Generate certificate %s -> %s", comp, host)))
			}
		}
	}
	return certificateTasks, nil
}

func buildInitMonitoredConfigTasks(
	specManager *spec.SpecManager,
	name string,
	uniqueHosts map[string]hostInfo, // host -> ssh-port, os, arch
	noAgentHosts set.StringSet,
	globalOptions spec.GlobalOptions,
	monitoredOptions *spec.MonitoredOptions,
	logger *logprinter.Logger,
	sshTimeout, exeTimeout uint64,
	gOpt operator.Options,
	p *tui.SSHConnectionProps,
) []*task.StepDisplay {
	if monitoredOptions == nil {
		return nil
	}

	tasks := []*task.StepDisplay{}
	// monitoring agents
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		for host, info := range uniqueHosts {
			if noAgentHosts.Exist(host) {
				continue
			}

			deployDir := spec.Abs(globalOptions.User, monitoredOptions.DeployDir)
			// data dir would be empty for components which don't need it
			dataDir := monitoredOptions.DataDir
			// the default data_dir is relative to deploy_dir
			if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join(deployDir, dataDir)
			}
			// log dir will always be with values, but might not used by the component
			logDir := spec.Abs(globalOptions.User, monitoredOptions.LogDir)
			// Generate configs

			t := task.NewSimpleUerSSH(logger, host, info.ssh, globalOptions.User, gOpt, p, globalOptions.SSHType).
				MonitoredConfig(
					name,
					comp,
					host,
					globalOptions.ResourceControl,
					monitoredOptions,
					globalOptions.User,
					globalOptions.TLSEnabled,
					meta.DirPaths{
						Deploy: deployDir,
						Data:   []string{dataDir},
						Log:    logDir,
						Cache:  specManager.Path(name, spec.TempConfigPath),
					},
					globalOptions.SystemdMode,
				).
				BuildAsStep(fmt.Sprintf("  - Generate config %s -> %s", comp, host))
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func buildInitConfigTasks(
	m *Manager,
	name string,
	topo spec.Topology,
	base *spec.BaseMeta,
	gOpt operator.Options,
	nodes []string,
) ([]*task.StepDisplay, bool) {
	var tasks []*task.StepDisplay
	hasImported := false
	deletedNodes := set.NewStringSet(nodes...)

	topo.IterInstance(func(instance spec.Instance) {
		if deletedNodes.Exist(instance.ID()) {
			return
		}
		compName := instance.ComponentName()
		deployDir := spec.Abs(base.User, instance.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := spec.MultiDirAbs(base.User, instance.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := spec.Abs(base.User, instance.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder(m.logger)
		if instance.IsImported() {
			version := instance.CalculateVersion(base.Version)
			switch compName {
			case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertmanager:
				tb.Download(compName, instance.OS(), instance.Arch(), version).
					CopyComponent(
						compName,
						instance.OS(),
						instance.Arch(),
						version,
						"", // use default srcPath
						instance.GetManageHost(),
						deployDir,
					)
			}
			hasImported = true
		}

		t := tb.
			InitConfig(
				name,
				base.Version,
				m.specManager,
				instance,
				base.User,
				gOpt.IgnoreConfigCheck,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  m.specManager.Path(name, spec.TempConfigPath),
				},
			).
			BuildAsStep(fmt.Sprintf("  - Generate config %s -> %s", compName, instance.ID()))
		tasks = append(tasks, t)
	})

	return tasks, hasImported
}

// buildDownloadCompTasks build download component tasks
func buildDownloadCompTasks(
	clusterVersion string,
	topo spec.Topology,
	logger *logprinter.Logger,
	gOpt operator.Options,
) []*task.StepDisplay {
	var tasks []*task.StepDisplay
	uniqueTaskList := set.NewStringSet()
	topo.IterInstance(func(inst spec.Instance) {
		key := fmt.Sprintf("%s-%s-%s", inst.ComponentSource(), inst.OS(), inst.Arch())
		if found := uniqueTaskList.Exist(key); !found {
			uniqueTaskList.Insert(key)

			// we don't set version for tispark, so the lastest tispark will be used
			var version string
			if inst.ComponentName() == spec.ComponentTiSpark {
				// download spark as dependency of tispark
				tasks = append(tasks, buildDownloadSparkTask(inst, logger, gOpt))
			} else {
				version = inst.CalculateVersion(clusterVersion)
			}

			t := task.NewBuilder(logger).
				Download(inst.ComponentSource(), inst.OS(), inst.Arch(), version).
				BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)",
					inst.ComponentSource(), version, inst.OS(), inst.Arch()))
			tasks = append(tasks, t)
		}
	})
	return tasks
}

// buildDownloadSparkTask build download task for spark, which is a dependency of tispark
// FIXME: this is a hack and should be replaced by dependency handling in manifest processing
func buildDownloadSparkTask(inst spec.Instance, logger *logprinter.Logger, gOpt operator.Options) *task.StepDisplay {
	return task.NewBuilder(logger).
		Download(spec.ComponentSpark, inst.OS(), inst.Arch(), "").
		BuildAsStep(fmt.Sprintf("  - Download %s: (%s/%s)",
			spec.ComponentSpark, inst.OS(), inst.Arch()))
}

// buildTLSTask create enable/disable tls task
func buildTLSTask(
	m *Manager,
	name string,
	metadata spec.Metadata,
	gOpt operator.Options,
	reloadCertificate bool,
	p *tui.SSHConnectionProps,
	delFileMap map[string]set.StringSet,
) (task.Task, error) {
	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	//  load certificate file
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		tlsDir := m.specManager.Path(name, spec.TLSCertKeyDir)
		m.logger.Infof("Generate certificate: %s", color.YellowString(tlsDir))
		if err := m.loadCertificate(name, topo.BaseTopo().GlobalOptions, reloadCertificate); err != nil {
			return nil, err
		}
	}

	certificateTasks, err := buildCertificateTasks(m, name, topo, base, gOpt, p)
	if err != nil {
		return nil, err
	}

	refreshConfigTasks, hasImported := buildInitConfigTasks(m, name, topo, base, gOpt, nil)

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(name); err != nil {
			return task.NewBuilder(m.logger).Build(), err
		}
	}

	// monitor
	uniqueHosts, noAgentHosts := getMonitorHosts(topo)
	moniterCertificateTasks, err := buildMonitoredCertificateTasks(
		m,
		name,
		uniqueHosts,
		noAgentHosts,
		topo.BaseTopo().GlobalOptions,
		topo.GetMonitoredOptions(),
		gOpt,
		p,
	)
	if err != nil {
		return nil, err
	}

	monitorConfigTasks := buildInitMonitoredConfigTasks(
		m.specManager,
		name,
		uniqueHosts,
		noAgentHosts,
		*topo.BaseTopo().GlobalOptions,
		topo.GetMonitoredOptions(),
		m.logger,
		gOpt.SSHTimeout,
		gOpt.OptTimeout,
		gOpt,
		p,
	)

	builder, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return nil, err
	}

	builder.
		ParallelStep("+ Copy certificate to remote host", gOpt.Force, certificateTasks...).
		ParallelStep("+ Copy monitor certificate to remote host", gOpt.Force, moniterCertificateTasks...).
		ParallelStep("+ Refresh instance configs", gOpt.Force, refreshConfigTasks...).
		ParallelStep("+ Refresh monitor configs", gOpt.Force, monitorConfigTasks...).
		Func("Save meta", func(_ context.Context) error {
			return m.specManager.SaveMeta(name, metadata)
		})

	// cleanup tls files only in tls disable
	if !topo.BaseTopo().GlobalOptions.TLSEnabled {
		builder.Func("Cleanup TLS files", func(ctx context.Context) error {
			return operator.CleanupComponent(ctx, delFileMap, topo.BaseTopo().GlobalOptions.SystemdMode != spec.UserMode)
		})
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return nil, err
	}

	builder.
		Func("Restart Cluster", func(ctx context.Context) error {
			return operator.Restart(ctx, topo, gOpt, tlsCfg)
		}).
		Func("Reload PD Members", func(ctx context.Context) error {
			return operator.SetPDMember(ctx, name, topo.BaseTopo().GlobalOptions.TLSEnabled, tlsCfg, metadata)
		})

	return builder.Build(), nil
}

func genTiProxySessionCerts(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	ca, err := crypto.NewCA("tiproxy")
	if err != nil {
		return err
	}
	privKey, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return err
	}
	csr, err := privKey.CSR("tiproxy", "tiproxy", nil, nil)
	if err != nil {
		return err
	}
	cert, err := ca.Sign(csr)
	if err != nil {
		return err
	}
	if err := utils.SaveFileWithBackup(filepath.Join(dir, "tiproxy-session.key"), privKey.Pem(), ""); err != nil {
		return err
	}
	return utils.SaveFileWithBackup(filepath.Join(dir, "tiproxy-session.crt"), pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}), "")
}

// buildSessionCertTasks puts a self-signed cert to all TiDB if there is tiproxy.
// For deploy: originalTopo = nil, newTopo = topology.
// For scale-out: originalTopo = original topology, newTopo = new topology.
func buildSessionCertTasks(m *Manager,
	name string,
	originalTopo spec.Topology,
	newTopo spec.Topology,
	base *spec.BaseMeta,
	gOpt operator.Options,
	p *tui.SSHConnectionProps) ([]*task.StepDisplay, error) {
	var certificateTasks []*task.StepDisplay // tasks which are used to copy certificate to remote host
	hasOriginalTiProxy := false
	if originalTopo != nil {
		originalTopo.IterInstance(func(inst spec.Instance) {
			if inst.ComponentName() == spec.ComponentTiProxy {
				hasOriginalTiProxy = true
			}
		})
	}
	hasNewTiProxy := false
	newTopo.IterInstance(func(inst spec.Instance) {
		if inst.ComponentName() == spec.ComponentTiProxy {
			hasNewTiProxy = true
		}
	})
	if !hasOriginalTiProxy && !hasNewTiProxy {
		return nil, nil
	}

	tempPath := m.specManager.Path(name, spec.TempConfigPath)
	keyPath := filepath.Join(tempPath, "tiproxy-session.key")
	certPath := filepath.Join(tempPath, "tiproxy-session.crt")
	copySessionCerts := func(inst spec.Instance) {
		if inst.ComponentName() != spec.ComponentTiDB {
			return
		}
		deployDir := spec.Abs(base.User, inst.DeployDir())
		tlsDir := filepath.Join(deployDir, spec.TLSCertKeyDir)

		tb := task.NewSimpleUerSSH(m.logger, inst.GetManageHost(), inst.GetSSHPort(), base.User, gOpt, p, newTopo.BaseTopo().GlobalOptions.SSHType).
			Mkdir(base.User, inst.GetManageHost(), newTopo.BaseTopo().GlobalOptions.SystemdMode != spec.UserMode, deployDir, tlsDir)
		tb = tb.
			CopyFile(keyPath, filepath.Join(deployDir, spec.TLSCertKeyDir, "tiproxy-session.key"), inst.GetHost(), false, 0, false).
			CopyFile(certPath, filepath.Join(deployDir, spec.TLSCertKeyDir, "tiproxy-session.crt"), inst.GetHost(), false, 0, false)
		t := tb.BuildAsStep(fmt.Sprintf("  - Copy session certificate %s -> %s", inst.ComponentName(), inst.ID()))
		certificateTasks = append(certificateTasks, t)
	}

	// If TiProxy is just enabled now (either deploy or scale-out), issue a session cert and copy the cert to original TiDB.
	if !hasOriginalTiProxy {
		if err := genTiProxySessionCerts(tempPath); err != nil {
			return certificateTasks, err
		}
		if originalTopo != nil {
			originalTopo.IterInstance(copySessionCerts)
		}
	}
	// Copy the session cert to new TiDB.
	newTopo.IterInstance(copySessionCerts)
	return certificateTasks, nil
}

// buildCertificateTasks generates certificate for instance and transfers it to the server
func buildCertificateTasks(
	m *Manager,
	name string,
	topo spec.Topology,
	base *spec.BaseMeta,
	gOpt operator.Options,
	p *tui.SSHConnectionProps) ([]*task.StepDisplay, error) {
	var (
		iterErr          error
		certificateTasks []*task.StepDisplay // tasks which are used to copy certificate to remote host
	)

	// copy TLS certificate to remote host
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		topo.IterInstance(func(inst spec.Instance) {
			deployDir := spec.Abs(base.User, inst.DeployDir())
			tlsDir := filepath.Join(deployDir, spec.TLSCertKeyDir)

			tb := task.NewSimpleUerSSH(m.logger, inst.GetManageHost(), inst.GetSSHPort(), base.User, gOpt, p, topo.BaseTopo().GlobalOptions.SSHType).
				Mkdir(base.User, inst.GetManageHost(), topo.BaseTopo().GlobalOptions.SystemdMode != spec.UserMode, deployDir, tlsDir)
			ca, err := crypto.ReadCA(
				name,
				m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSCACert),
				m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSCAKey),
			)
			if err != nil {
				iterErr = err
				return
			}
			tb = tb.TLSCert(
				inst.GetHost(),
				inst.ComponentName(),
				inst.Role(),
				inst.GetMainPort(),
				ca,
				meta.DirPaths{
					Deploy: deployDir,
					Cache:  m.specManager.Path(name, spec.TempConfigPath),
				})

			t := tb.BuildAsStep(fmt.Sprintf("  - Generate certificate %s -> %s", inst.ComponentName(), inst.ID()))
			certificateTasks = append(certificateTasks, t)
		})
	}

	return certificateTasks, iterErr
}
