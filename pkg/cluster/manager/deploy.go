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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	pkgver "github.com/pingcap/tiup/pkg/repository/version"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
)

// DeployOptions contains the options for scale out.
// TODO: merge ScaleOutOptions, should check config too when scale out.
type DeployOptions struct {
	User              string // username to login to the SSH server
	SkipCreateUser    bool   // don't create the user
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
	NoLabels          bool   // don't check labels for TiKV instance
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
	afterDeploy func(b *task.Builder, newPart spec.Topology),
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
			WithProperty(cliutil.SuggestionFromFormat("Please specify another cluster name"))
	}

	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := spec.ParseTopologyYaml(topoFile, topo); err != nil {
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

	if topo, ok := topo.(*spec.Specification); ok && !opt.NoLabels {
		// Check if TiKV's label set correctly
		lbs, err := topo.LocationLabels()
		if err != nil {
			return err
		}
		if err := spec.CheckTiKVLabels(lbs, topo); err != nil {
			return perrs.Errorf("check TiKV label failed, please fix that before continue:\n%s", err)
		}
	}

	clusterList, err := m.specManager.GetAllClusters()
	if err != nil {
		return err
	}
	if err := spec.CheckClusterPortConflict(clusterList, name, topo); err != nil {
		return err
	}
	if err := spec.CheckClusterDirConflict(clusterList, name, topo); err != nil {
		return err
	}

	if !skipConfirm {
		if err := m.confirmTopology(name, clusterVersion, topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	var sshConnProps *cliutil.SSHConnectionProps = &cliutil.SSHConnectionProps{}
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = cliutil.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(m.specManager.Path(name), 0755); err != nil {
		return errorx.InitializationFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", m.specManager.Path(name)).
			WithProperty(cliutil.SuggestionFromString("Please check file system permissions and try again."))
	}

	var (
		envInitTasks      []*task.StepDisplay // tasks which are used to initialize environment
		downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	// Initialize environment
	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	globalOptions := base.GlobalOptions

	// generate CA and client cert for TLS enabled cluster
	var ca *crypto.CertificateAuthority
	if globalOptions.TLSEnabled {
		// generate CA
		tlsPath := m.specManager.Path(name, spec.TLSCertKeyDir)
		if err := utils.CreateDir(tlsPath); err != nil {
			return err
		}
		ca, err = genAndSaveClusterCA(name, tlsPath)
		if err != nil {
			return err
		}

		// generate client cert
		if err = genAndSaveClientCert(ca, name, tlsPath); err != nil {
			return err
		}
	}

	var iterErr error // error when itering over instances
	iterErr = nil
	topo.IterInstance(func(inst spec.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			// check for "imported" parameter, it can not be true when scaling out
			if inst.IsImported() {
				iterErr = errors.New(
					"'imported' is set to 'true' for new instance, this is only used " +
						"for instances imported from tidb-ansible and make no sense when " +
						"deploying new instances, please delete the line or set it to 'false' for new instances")
				return // skip the host to avoid issues
			}

			uniqueHosts[inst.GetHost()] = hostInfo{
				ssh:  inst.GetSSHPort(),
				os:   inst.OS(),
				arch: inst.Arch(),
			}
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
			t := task.NewBuilder().
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					opt.User,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					gOpt.SSHTimeout,
					gOpt.SSHType,
					globalOptions.SSHType,
				).
				EnvInit(inst.GetHost(), globalOptions.User, globalOptions.Group, opt.SkipCreateUser || globalOptions.User == opt.User).
				Mkdir(globalOptions.User, inst.GetHost(), dirs...).
				BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", inst.GetHost(), inst.GetSSHPort()))
			envInitTasks = append(envInitTasks, t)
		}
	})

	if iterErr != nil {
		return iterErr
	}

	// Download missing component
	downloadCompTasks = buildDownloadCompTasks(clusterVersion, topo, m.bindVersion)

	// Deploy components to remote
	topo.IterInstance(func(inst spec.Instance) {
		version := m.bindVersion(inst.ComponentName(), clusterVersion)
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
		if globalOptions.TLSEnabled {
			deployDirs = append(deployDirs, filepath.Join(deployDir, "tls"))
		}
		t := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), globalOptions.User, gOpt.SSHTimeout, gOpt.SSHType, globalOptions.SSHType).
			Mkdir(globalOptions.User, inst.GetHost(), deployDirs...).
			Mkdir(globalOptions.User, inst.GetHost(), dataDirs...)

		if deployerInstance, ok := inst.(DeployerInstance); ok {
			deployerInstance.Deploy(t, "", deployDir, version, name, clusterVersion)
		} else {
			// copy dependency component if needed
			switch inst.ComponentName() {
			case spec.ComponentTiSpark:
				env := environment.GlobalEnv()
				var sparkVer pkgver.Version
				if sparkVer, _, iterErr = env.V1Repository().LatestStableVersion(spec.ComponentSpark, false); iterErr != nil {
					return
				}
				t = t.DeploySpark(inst, sparkVer.String(), "" /* default srcPath */, deployDir)
			default:
				t = t.CopyComponent(
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

		// generate and transfer tls cert for instance
		if globalOptions.TLSEnabled {
			t = t.TLSCert(inst, ca, meta.DirPaths{
				Deploy: deployDir,
				Cache:  m.specManager.Path(name, spec.TempConfigPath),
			})
		}

		// generate configs for the component
		t = t.InitConfig(
			name,
			clusterVersion,
			m.specManager,
			inst,
			globalOptions.User,
			opt.IgnoreConfigCheck,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  m.specManager.Path(name, spec.TempConfigPath),
			},
		)

		deployCompTasks = append(deployCompTasks,
			t.BuildAsStep(fmt.Sprintf("  - Copy %s -> %s", inst.ComponentName(), inst.GetHost())),
		)
	})

	if iterErr != nil {
		return iterErr
	}

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(
		m.bindVersion,
		m.specManager,
		name,
		uniqueHosts,
		globalOptions,
		topo.GetMonitoredOptions(),
		clusterVersion,
		gOpt,
	)
	downloadCompTasks = append(downloadCompTasks, dlTasks...)
	deployCompTasks = append(deployCompTasks, dpTasks...)

	builder := task.NewBuilder().
		Step("+ Generate SSH keys",
			task.NewBuilder().SSHKeyGen(m.specManager.Path(name, "ssh", "id_rsa")).Build()).
		ParallelStep("+ Download TiDB components", false, downloadCompTasks...).
		ParallelStep("+ Initialize target host environments", false, envInitTasks...).
		ParallelStep("+ Copy files", false, deployCompTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, topo)
	}

	t := builder.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}

	metadata.SetUser(globalOptions.User)
	metadata.SetVersion(clusterVersion)
	err = m.specManager.SaveMeta(name, metadata)

	if err != nil {
		return err
	}

	hint := color.New(color.Bold).Sprintf("%s start %s", cliutil.OsArgs0(), name)
	log.Infof("Cluster `%s` deployed successfully, you can start it with command: `%s`", name, hint)
	return nil
}
