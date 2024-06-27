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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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

	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

// DeployOptions contains the options for scale out.
type DeployOptions struct {
	User           string // username to login to the SSH server
	SkipCreateUser bool   // don't create the user
	IdentityFile   string // path to the private key file
	UsePassword    bool   // use password instead of identity file for ssh connection
	NoLabels       bool   // don't check labels for TiKV instance
	Stage1         bool   // don't start the new instance, just deploy
	Stage2         bool   // start instances and init Config after stage1
}

// DeployerInstance is a instance can deploy to a target deploy directory.
type DeployerInstance interface {
	Deploy(b *task.Builder, srcPath string, deployDir string, version string, name string, clusterVersion string)
}

// Deploy a cluster.
func (m *Manager) Deploy(
	name string,
	clusterVersion string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology, gOpt operator.Options),
	skipConfirm bool,
	gOpt operator.Options,
) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	exist, err := m.specManager.Exist(name)
	if err != nil {
		return err
	}

	if exist {
		// FIXME: When change to use args, the suggestion text need to be updatem.
		return errDeployNameDuplicate.
			New("Cluster name '%s' is duplicated", name).
			WithProperty(tui.SuggestionFromFormat("Please specify another cluster name"))
	}

	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}

	if err := checkTiFlashWithTLS(topo, clusterVersion); err != nil {
		return err
	}

	instCnt := 0
	topo.IterInstance(func(inst spec.Instance) {
		switch inst.ComponentName() {
		// monitoring components are only useful when deployed with
		// core components, we do not support deploying any bare
		// monitoring system.
		case spec.ComponentGrafana,
			spec.ComponentPrometheus,
			spec.ComponentAlertmanager:
			return
		}
		instCnt++
	})
	if instCnt < 1 {
		return fmt.Errorf("no valid instance found in the input topology, please check your config")
	}

	spec.ExpandRelativeDir(topo)

	base := topo.BaseTopo()
	if sshType := gOpt.SSHType; sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	if topo, ok := topo.(*spec.Specification); ok {
		topo.AdjustByVersion(clusterVersion)
		if !opt.NoLabels {
			// Check if TiKV's label set correctly
			lbs, err := topo.LocationLabels()
			if err != nil {
				return err
			}
			if err := spec.CheckTiKVLabels(lbs, topo); err != nil {
				return perrs.Errorf("check TiKV label failed, please fix that before continue:\n%s", err)
			}
		}
	}

	if err := checkConflict(m, name, topo); err != nil {
		return err
	}

	var (
		sshConnProps  *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
		sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	)
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = tui.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
		if len(gOpt.SSHProxyHost) != 0 {
			if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
				return err
			}
		}
	}

	var sudo bool
	systemdMode := topo.BaseTopo().GlobalOptions.SystemdMode
	if systemdMode == spec.UserMode {
		sudo = false
		hint := fmt.Sprintf("loginctl enable-linger %s", opt.User)

		msg := "The value of systemd_mode is set to `user` in the topology, please note that you'll need to manually execute the following command using root or sudo on the host(s) to enable lingering for the systemd user instance.\n"
		msg += color.GreenString(hint)
		msg += "\nYou can read the systemd documentation for reference: https://wiki.archlinux.org/title/Systemd/User#Automatic_start-up_of_systemd_user_instances."
		m.logger.Warnf(msg)
		err = tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
		if err != nil {
			return err
		}
	} else {
		sudo = true
	}

	if err := m.fillHost(sshConnProps, sshProxyProps, topo, &gOpt, opt.User, opt.User != "root" && systemdMode != spec.UserMode); err != nil {
		return err
	}

	if !skipConfirm && strings.ToLower(gOpt.DisplayMode) != "json" {
		if err := m.confirmTopology(name, clusterVersion, topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	if err := utils.MkdirAll(m.specManager.Path(name), 0755); err != nil {
		return errorx.InitializationFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", m.specManager.Path(name)).
			WithProperty(tui.SuggestionFromString("Please check file system permissions and try again."))
	}

	var (
		envInitTasks      []*task.StepDisplay // tasks which are used to initialize environment
		downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	// Initialize environment

	globalOptions := base.GlobalOptions

	metadata.SetUser(globalOptions.User)
	metadata.SetVersion(clusterVersion)

	var iterErr error // error when itering over instances
	iterErr = nil

	topo.IterInstance(func(inst spec.Instance) {
		// check for "imported" parameter, it can not be true when deploying and scaling out
		// only for tidb now, need to support dm
		if inst.IsImported() && m.sysName == "tidb" {
			iterErr = errors.New(
				"'imported' is set to 'true' for new instance, this is only used " +
					"for instances imported from tidb-ansible and make no sense when " +
					"deploying new instances, please delete the line or set it to 'false' for new instances")
			return // skip the host to avoid issues
		}
	})

	// generate CA and client cert for TLS enabled cluster
	_, err = m.genAndSaveCertificate(name, globalOptions)
	if err != nil {
		return err
	}

	uniqueHosts, noAgentHosts := getMonitorHosts(topo)

	for host, hostInfo := range uniqueHosts {
		var dirs []string
		for _, dir := range []string{globalOptions.DeployDir, globalOptions.LogDir} {
			if dir == "" {
				continue
			}

			dirs = append(dirs, spec.Abs(globalOptions.User, dir))
		}
		// the default, relative path of data dir is under deploy dir
		if strings.HasPrefix(globalOptions.DataDir, "/") {
			dirs = append(dirs, globalOptions.DataDir)
		}
		if systemdMode == spec.UserMode {
			dirs = append(dirs, spec.Abs(globalOptions.User, ".config/systemd/user"))
		}
		t := task.NewBuilder(m.logger).
			RootSSH(
				host,
				hostInfo.ssh,
				opt.User,
				sshConnProps.Password,
				sshConnProps.IdentityFile,
				sshConnProps.IdentityFilePassphrase,
				gOpt.SSHTimeout,
				gOpt.OptTimeout,
				gOpt.SSHProxyHost,
				gOpt.SSHProxyPort,
				gOpt.SSHProxyUser,
				sshProxyProps.Password,
				sshProxyProps.IdentityFile,
				sshProxyProps.IdentityFilePassphrase,
				gOpt.SSHProxyTimeout,
				gOpt.SSHType,
				globalOptions.SSHType,
				opt.User != "root" && systemdMode != spec.UserMode,
			).
			EnvInit(host, globalOptions.User, globalOptions.Group, opt.SkipCreateUser || globalOptions.User == opt.User, sudo).
			Mkdir(globalOptions.User, host, sudo, dirs...).
			BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", host, hostInfo.ssh))
		envInitTasks = append(envInitTasks, t)
	}

	if iterErr != nil {
		return iterErr
	}

	// Download missing component
	downloadCompTasks = buildDownloadCompTasks(clusterVersion, topo, m.logger, gOpt)

	// Deploy components to remote
	topo.IterInstance(func(inst spec.Instance) {
		version := inst.CalculateVersion(clusterVersion)
		deployDir := spec.Abs(globalOptions.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := spec.MultiDirAbs(globalOptions.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := spec.Abs(globalOptions.User, inst.LogDir())
		// Deploy component
		// prepare deployment server
		deployDirs := []string{
			deployDir, logDir,
			filepath.Join(deployDir, "bin"),
			filepath.Join(deployDir, "conf"),
			filepath.Join(deployDir, "scripts"),
		}

		t := task.NewSimpleUerSSH(m.logger, inst.GetManageHost(), inst.GetSSHPort(), globalOptions.User, gOpt, sshProxyProps, globalOptions.SSHType).
			Mkdir(globalOptions.User, inst.GetManageHost(), sudo, deployDirs...).
			Mkdir(globalOptions.User, inst.GetManageHost(), sudo, dataDirs...)

		if deployerInstance, ok := inst.(DeployerInstance); ok {
			deployerInstance.Deploy(t, "", deployDir, version, name, clusterVersion)
		} else {
			// copy dependency component if needed
			switch inst.ComponentName() {
			case spec.ComponentTiSpark:
				env := environment.GlobalEnv()
				var sparkVer utils.Version
				if sparkVer, _, iterErr = env.V1Repository().WithOptions(repository.Options{
					GOOS:   inst.OS(),
					GOARCH: inst.Arch(),
				}).LatestStableVersion(spec.ComponentSpark, false); iterErr != nil {
					return
				}
				t = t.DeploySpark(inst, sparkVer.String(), "" /* default srcPath */, deployDir)
			default:
				t = t.CopyComponent(
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

		deployCompTasks = append(deployCompTasks,
			t.BuildAsStep(fmt.Sprintf("  - Copy %s -> %s", inst.ComponentName(), inst.GetManageHost())),
		)
	})

	if iterErr != nil {
		return iterErr
	}

	// generates certificate for instance and transfers it to the server
	certificateTasks, err := buildCertificateTasks(m, name, topo, metadata.GetBaseMeta(), gOpt, sshProxyProps)
	if err != nil {
		return err
	}
	sessionCertTasks, err := buildSessionCertTasks(m, name, nil, topo, metadata.GetBaseMeta(), gOpt, sshProxyProps)
	if err != nil {
		return err
	}
	certificateTasks = append(certificateTasks, sessionCertTasks...)

	refreshConfigTasks, _ := buildInitConfigTasks(m, name, topo, metadata.GetBaseMeta(), gOpt, nil)

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks, err := buildMonitoredDeployTask(
		m,
		uniqueHosts,
		noAgentHosts,
		globalOptions,
		topo.GetMonitoredOptions(),
		gOpt,
		sshProxyProps,
	)
	if err != nil {
		return err
	}
	downloadCompTasks = append(downloadCompTasks, dlTasks...)
	deployCompTasks = append(deployCompTasks, dpTasks...)

	// monitor tls file
	moniterCertificateTasks, err := buildMonitoredCertificateTasks(
		m,
		name,
		uniqueHosts,
		noAgentHosts,
		topo.BaseTopo().GlobalOptions,
		topo.GetMonitoredOptions(),
		gOpt,
		sshProxyProps,
	)
	if err != nil {
		return err
	}
	certificateTasks = append(certificateTasks, moniterCertificateTasks...)

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
		sshProxyProps,
	)
	builder := task.NewBuilder(m.logger).
		Step("+ Generate SSH keys",
			task.NewBuilder(m.logger).
				SSHKeyGen(m.specManager.Path(name, "ssh", "id_rsa")).
				Build(),
			m.logger).
		ParallelStep("+ Download TiDB components", false, downloadCompTasks...).
		ParallelStep("+ Initialize target host environments", false, envInitTasks...).
		ParallelStep("+ Deploy TiDB instance", false, deployCompTasks...).
		ParallelStep("+ Copy certificate to remote host", gOpt.Force, certificateTasks...).
		ParallelStep("+ Init instance configs", gOpt.Force, refreshConfigTasks...).
		ParallelStep("+ Init monitor configs", gOpt.Force, monitorConfigTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, topo, gOpt)
	}

	t := builder.Build()

	ctx := ctxt.New(
		context.Background(),
		gOpt.Concurrency,
		m.logger,
	)
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	err = m.specManager.SaveMeta(name, metadata)

	if err != nil {
		return err
	}

	var hint string
	if topo.Type() == spec.TopoTypeTiDB {
		hint = color.New(color.Bold).Sprintf("%s start %s --init", tui.OsArgs0(), name)
	} else {
		hint = color.New(color.Bold).Sprintf("%s start %s", tui.OsArgs0(), name)
	}
	m.logger.Infof("Cluster `%s` deployed successfully, you can start it with command: `%s`", name, hint)
	return nil
}
