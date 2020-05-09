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
	"path/filepath"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil/prepare"
	"github.com/pingcap-incubator/tiup-cluster/pkg/clusterutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/logger"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	operator "github.com/pingcap-incubator/tiup-cluster/pkg/operation"
	"github.com/pingcap-incubator/tiup-cluster/pkg/task"
	"github.com/pingcap-incubator/tiup-cluster/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type scaleOutOptions struct {
	user         string // username to login to the SSH server
	identityFile string // path to the private key file
	usePassword  bool   // use password instead of identity file for ssh connection
}

func newScaleOutCmd() *cobra.Command {
	opt := scaleOutOptions{
		identityFile: filepath.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "scale-out <cluster-name> <topology.yaml>",
		Short:        "Scale out a TiDB cluster",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			logger.EnableAuditLog()
			return scaleOut(args[0], args[1], opt)
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.identityFile, "identity_file", "i", opt.identityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.usePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")

	return cmd
}

func scaleOut(clusterName, topoFile string, opt scaleOutOptions) error {
	if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
		return errors.Errorf("cannot scale-out non-exists cluster %s", clusterName)
	}

	var newPart meta.TopologySpecification
	if err := utils.ParseTopologyYaml(topoFile, &newPart); err != nil {
		return err
	}

	metadata, err := meta.ClusterMetadata(clusterName)
	if err != nil {
		return err
	}

	// Abort scale out operation if the merged topology is invalid
	mergedTopo := metadata.Topology.Merge(&newPart)
	if err := mergedTopo.Validate(); err != nil {
		return err
	}

	if err := prepare.CheckClusterPortConflict(clusterName, mergedTopo); err != nil {
		return err
	}
	if err := prepare.CheckClusterDirConflict(clusterName, mergedTopo); err != nil {
		return err
	}

	patchedComponents := set.NewStringSet()
	newPart.IterInstance(func(instance meta.Instance) {
		if exists := tiuputils.IsExist(meta.ClusterPath(clusterName, meta.PatchDirName, instance.ComponentName()+".tar.gz")); exists {
			patchedComponents.Insert(instance.ComponentName())
		}
	})
	if !skipConfirm {
		// patchedComponents are components that have been patched and overwrited
		if err := confirmTopology(clusterName, metadata.Version, &newPart, patchedComponents); err != nil {
			return err
		}
	}

	// Inherit existing global configuration
	newPart.GlobalOptions = metadata.Topology.GlobalOptions
	newPart.MonitoredOptions = metadata.Topology.MonitoredOptions
	newPart.ServerConfigs = metadata.Topology.ServerConfigs

	sshConnProps, err := cliutil.ReadIdentityFileOrPassword(opt.identityFile, opt.usePassword)
	if err != nil {
		return err
	}

	// Build the scale out tasks
	t, err := buildScaleOutTask(clusterName, metadata, mergedTopo, opt, sshConnProps, &newPart, patchedComponents)
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

	log.Infof("Scaled cluster `%s` out successfully", clusterName)

	return nil
}

// Deprecated
func convertStepDisplaysToTasks(t []*task.StepDisplay) []task.Task {
	tasks := make([]task.Task, 0, len(t))
	for _, sd := range t {
		tasks = append(tasks, sd)
	}
	return tasks
}

func buildScaleOutTask(
	clusterName string,
	metadata *meta.ClusterMeta,
	mergedTopo *meta.ClusterSpecification,
	opt scaleOutOptions,
	sshConnProps *cliutil.SSHConnectionProps,
	newPart *meta.TopologySpecification,
	patchedComponents set.StringSet) (task.Task, error) {
	var (
		envInitTasks       []task.Task // tasks which are used to initialize environment
		downloadCompTasks  []task.Task // tasks which are used to download components
		deployCompTasks    []task.Task // tasks which are used to copy components to remote host
		refreshConfigTasks []task.Task // tasks which are used to refresh configuration
	)

	// Initialize the environments
	initializedHosts := set.NewStringSet()
	metadata.Topology.IterInstance(func(instance meta.Instance) {
		initializedHosts.Insert(instance.GetHost())
	})
	// uninitializedHosts are hosts which haven't been initialized yet
	uninitializedHosts := map[string]int{} // host -> ssh-port
	var iterErr error                      // error when itering over instances
	iterErr = nil
	newPart.IterInstance(func(instance meta.Instance) {
		if host := instance.GetHost(); !initializedHosts.Exist(host) {
			// check for "imported" parameter, it can not be true when scaling out
			if instance.IsImported() {
				iterErr = errors.New(
					"'imported' is set to 'true' for new instance, this is only used " +
						"for instances imported from tidb-ansible and make no sense when " +
						"scaling out, please delete the line or set it to 'false' for new instances")
				return // skip the host to avoid issues
			}

			uninitializedHosts[host] = instance.GetSSHPort()

			var dirs []string
			globalOptions := metadata.Topology.GlobalOptions
			for _, dir := range []string{globalOptions.DeployDir, globalOptions.DataDir, globalOptions.LogDir} {
				if dir == "" {
					continue
				}
				dirs = append(dirs, clusterutil.Abs(globalOptions.User, dir))
			}
			t := task.NewBuilder().
				RootSSH(
					instance.GetHost(),
					instance.GetSSHPort(),
					opt.user,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					sshTimeout,
				).
				EnvInit(instance.GetHost(), metadata.User).
				Mkdir(globalOptions.User, instance.GetHost(), dirs...).
				Build()
			envInitTasks = append(envInitTasks, t)
		}
	})

	if iterErr != nil {
		return task.NewBuilder().Build(), iterErr
	}

	// Download missing component
	downloadCompTasks = convertStepDisplaysToTasks(prepare.BuildDownloadCompTasks(metadata.Version, newPart))

	// Deploy the new topology and refresh the configuration
	newPart.IterInstance(func(inst meta.Instance) {
		version := meta.ComponentVersion(inst.ComponentName(), metadata.Version)
		deployDir := clusterutil.Abs(metadata.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDir := clusterutil.Abs(metadata.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(metadata.User, inst.LogDir())

		// Deploy component
		tb := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), metadata.User, sshTimeout).
			Mkdir(metadata.User, inst.GetHost(),
				deployDir, dataDir, logDir,
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts"))
		if patchedComponents.Exist(inst.ComponentName()) {
			tb.InstallPackage(meta.ClusterPath(clusterName, meta.PatchDirName, inst.ComponentName()+".tar.gz"), inst.GetHost(), deployDir)
		} else {
			tb.CopyComponent(inst.ComponentName(), version, inst.GetHost(), deployDir)
		}
		t := tb.ScaleConfig(clusterName,
			metadata.Version,
			metadata.Topology,
			inst,
			metadata.User,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDir,
				Log:    logDir,
			},
		).Build()
		deployCompTasks = append(deployCompTasks, t)
	})

	mergedTopo.IterInstance(func(inst meta.Instance) {
		deployDir := clusterutil.Abs(metadata.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDir := clusterutil.Abs(metadata.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(metadata.User, inst.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder()
		if inst.IsImported() {
			switch compName := inst.ComponentName(); compName {
			case meta.ComponentGrafana, meta.ComponentPrometheus, meta.ComponentAlertManager:
				version := meta.ComponentVersion(compName, metadata.Version)
				tb.Download(compName, version).CopyComponent(compName, version, inst.GetHost(), deployDir)
			}
		}

		// Refresh all configuration
		t := tb.InitConfig(clusterName,
			metadata.Version,
			inst,
			metadata.User,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDir,
				Log:    logDir,
				Cache:  meta.ClusterPath(clusterName, "config"),
			},
		).Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(
		clusterName,
		uninitializedHosts,
		metadata.Topology.GlobalOptions,
		metadata.Topology.MonitoredOptions,
		metadata.Version,
	)
	downloadCompTasks = append(downloadCompTasks, convertStepDisplaysToTasks(dlTasks)...)
	deployCompTasks = append(deployCompTasks, convertStepDisplaysToTasks(dpTasks)...)

	return task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(clusterName, "ssh", "id_rsa"),
			meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		Parallel(downloadCompTasks...).
		Parallel(envInitTasks...).
		Parallel(deployCompTasks...).
		// TODO: find another way to make sure current cluster started
		ClusterSSH(metadata.Topology, metadata.User, sshTimeout).
		ClusterOperate(metadata.Topology, operator.StartOperation, operator.Options{}).
		ClusterSSH(newPart, metadata.User, sshTimeout).
		Func("save meta", func() error {
			metadata.Topology = mergedTopo
			return meta.SaveClusterMeta(clusterName, metadata)
		}).
		ClusterOperate(newPart, operator.StartOperation, operator.Options{}).
		Parallel(refreshConfigTasks...).
		ClusterOperate(metadata.Topology, operator.RestartOperation, operator.Options{Roles: []string{meta.ComponentPrometheus}}).
		Build(), nil

}
