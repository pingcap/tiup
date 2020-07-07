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
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cliutil/prepare"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/report"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type scaleOutOptions struct {
	user         string // username to login to the SSH server
	identityFile string // path to the private key file
	usePassword  bool   // use password instead of identity file for ssh connection
}

func newScaleOutCmd() *cobra.Command {
	opt := scaleOutOptions{
		identityFile: filepath.Join(tiuputils.UserHome(), ".ssh", "id_rsa"),
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
			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))
			return scaleOut(args[0], args[1], opt)
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", tiuputils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.identityFile, "identity_file", "i", opt.identityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.usePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")

	return cmd
}

func scaleOut(clusterName, topoFile string, opt scaleOutOptions) error {
	exist, err := tidbSpec.Exist(clusterName)
	if err != nil {
		return errors.AddStack(err)
	}

	if !exist {
		return errors.Errorf("cannot scale-out non-exists cluster %s", clusterName)
	}

	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil { // not allowing validation errors
		return err
	}

	// Inherit existing global configuration. We must assign the inherited values before unmarshalling
	// because some default value rely on the global options and monitored options.
	var newPart = spec.Specification{
		GlobalOptions:    metadata.Topology.GlobalOptions,
		MonitoredOptions: metadata.Topology.MonitoredOptions,
		ServerConfigs:    metadata.Topology.ServerConfigs,
	}
	if err := clusterutil.ParseTopologyYaml(topoFile, &newPart); err != nil {
		return err
	}

	if err := validateNewTopo(&newPart); err != nil {
		return err
	}

	if data, err := ioutil.ReadFile(topoFile); err == nil {
		teleTopology = string(data)
	}

	// Abort scale out operation if the merged topology is invalid
	mergedTopo := metadata.Topology.Merge(&newPart)
	if err := mergedTopo.Validate(); err != nil {
		return err
	}

	if err := prepare.CheckClusterPortConflict(tidbSpec, clusterName, mergedTopo); err != nil {
		return err
	}
	if err := prepare.CheckClusterDirConflict(tidbSpec, clusterName, mergedTopo); err != nil {
		return err
	}

	patchedComponents := set.NewStringSet()
	newPart.IterInstance(func(instance spec.Instance) {
		if tiuputils.IsExist(spec.ClusterPath(clusterName, spec.PatchDirName, instance.ComponentName()+".tar.gz")) {
			patchedComponents.Insert(instance.ComponentName())
		}
	})

	if !skipConfirm {
		// patchedComponents are components that have been patched and overwrited
		if err := confirmTopology(clusterName, metadata.Version, &newPart, patchedComponents); err != nil {
			return err
		}
	}

	sshConnProps, err := cliutil.ReadIdentityFileOrPassword(opt.identityFile, opt.usePassword)
	if err != nil {
		return err
	}

	// Build the scale out tasks
	t, err := buildScaleOutTask(clusterName, metadata, mergedTopo, opt, sshConnProps, &newPart, patchedComponents, gOpt.OptTimeout)
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
	metadata *spec.ClusterMeta,
	mergedTopo *spec.Specification,
	opt scaleOutOptions,
	sshConnProps *cliutil.SSHConnectionProps,
	newPart *spec.Specification,
	patchedComponents set.StringSet,
	timeout int64,
) (task.Task, error) {
	var (
		envInitTasks       []task.Task // tasks which are used to initialize environment
		downloadCompTasks  []task.Task // tasks which are used to download components
		deployCompTasks    []task.Task // tasks which are used to copy components to remote host
		refreshConfigTasks []task.Task // tasks which are used to refresh configuration
	)

	// Initialize the environments
	initializedHosts := set.NewStringSet()
	metadata.Topology.IterInstance(func(instance spec.Instance) {
		initializedHosts.Insert(instance.GetHost())
	})
	// uninitializedHosts are hosts which haven't been initialized yet
	uninitializedHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	newPart.IterInstance(func(instance spec.Instance) {
		if host := instance.GetHost(); !initializedHosts.Exist(host) {
			if _, found := uninitializedHosts[host]; found {
				return
			}

			uninitializedHosts[host] = hostInfo{
				ssh:  instance.GetSSHPort(),
				os:   instance.OS(),
				arch: instance.Arch(),
			}

			var dirs []string
			globalOptions := metadata.Topology.GlobalOptions
			for _, dir := range []string{globalOptions.DeployDir, globalOptions.DataDir, globalOptions.LogDir} {
				for _, dirname := range strings.Split(dir, ",") {
					if dirname == "" {
						continue
					}
					dirs = append(dirs, clusterutil.Abs(globalOptions.User, dirname))
				}
			}
			t := task.NewBuilder().
				RootSSH(
					instance.GetHost(),
					instance.GetSSHPort(),
					opt.user,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					gOpt.SSHTimeout,
				).
				EnvInit(instance.GetHost(), metadata.User).
				Mkdir(globalOptions.User, instance.GetHost(), dirs...).
				Build()
			envInitTasks = append(envInitTasks, t)
		}
	})

	// Download missing component
	downloadCompTasks = convertStepDisplaysToTasks(prepare.BuildDownloadCompTasks(metadata.Version, newPart))

	// Deploy the new topology and refresh the configuration
	newPart.IterInstance(func(inst spec.Instance) {
		version := spec.ComponentVersion(inst.ComponentName(), metadata.Version)
		deployDir := clusterutil.Abs(metadata.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(metadata.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(metadata.User, inst.LogDir())

		// Deploy component
		tb := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), metadata.User, gOpt.SSHTimeout).
			Mkdir(metadata.User, inst.GetHost(),
				deployDir, logDir,
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts")).
			Mkdir(metadata.User, inst.GetHost(), dataDirs...)
		if patchedComponents.Exist(inst.ComponentName()) {
			tb.InstallPackage(spec.ClusterPath(clusterName, spec.PatchDirName, inst.ComponentName()+".tar.gz"), inst.GetHost(), deployDir)
		} else {
			tb.CopyComponent(inst.ComponentName(), inst.OS(), inst.Arch(), version, inst.GetHost(), deployDir)
		}
		t := tb.ScaleConfig(clusterName,
			metadata.Version,
			metadata.Topology,
			inst,
			metadata.User,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
			},
		).Build()
		deployCompTasks = append(deployCompTasks, t)
	})

	hasImported := false

	mergedTopo.IterInstance(func(inst spec.Instance) {
		deployDir := clusterutil.Abs(metadata.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(metadata.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(metadata.User, inst.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder()
		if inst.IsImported() {
			switch compName := inst.ComponentName(); compName {
			case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertManager:
				version := spec.ComponentVersion(compName, metadata.Version)
				tb.Download(compName, inst.OS(), inst.Arch(), version).
					CopyComponent(compName, inst.OS(), inst.Arch(), version, inst.GetHost(), deployDir)
			}
			hasImported = true
		}

		// Refresh all configuration
		t := tb.InitConfig(clusterName,
			metadata.Version,
			inst,
			metadata.User,
			true, // always ignore config check result in scale out
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  spec.ClusterPath(clusterName, spec.TempConfigPath),
			},
		).Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return task.NewBuilder().Build(), err
		}
	}

	nodeInfoTask := task.NewBuilder().Func("Check status", func(ctx *task.Context) error {
		var err error
		teleNodeInfos, err = operator.GetNodeInfo(context.Background(), ctx, newPart)
		_ = err
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)

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

	builder := task.NewBuilder().
		SSHKeySet(
			spec.ClusterPath(clusterName, "ssh", "id_rsa"),
			spec.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		Parallel(downloadCompTasks...).
		Parallel(envInitTasks...).
		ClusterSSH(metadata.Topology, metadata.User, gOpt.SSHTimeout).
		Parallel(deployCompTasks...)

	if report.Enable() {
		builder.Parallel(convertStepDisplaysToTasks([]*task.StepDisplay{nodeInfoTask})...)
	}

	// TODO: find another way to make sure current cluster started
	builder.ClusterOperate(metadata.Topology, operator.StartOperation, operator.Options{OptTimeout: timeout}).
		ClusterSSH(newPart, metadata.User, gOpt.SSHTimeout).
		Func("save meta", func(_ *task.Context) error {
			metadata.Topology = mergedTopo
			return spec.SaveClusterMeta(clusterName, metadata)
		}).
		ClusterOperate(newPart, operator.StartOperation, operator.Options{OptTimeout: timeout}).
		Parallel(refreshConfigTasks...).
		ClusterOperate(metadata.Topology, operator.RestartOperation, operator.Options{
			Roles:      []string{spec.ComponentPrometheus},
			OptTimeout: timeout,
		}).
		UpdateTopology(clusterName, metadata, nil)

	return builder.Build(), nil

}

// validateNewTopo checks the new part of scale-out topology to make sure it's supported
func validateNewTopo(topo *spec.Specification) (err error) {
	err = nil
	topo.IterInstance(func(instance spec.Instance) {
		// check for "imported" parameter, it can not be true when scaling out
		if instance.IsImported() {
			err = errors.New(
				"'imported' is set to 'true' for new instance, this is only used " +
					"for instances imported from tidb-ansible and make no sense when " +
					"scaling out, please delete the line or set it to 'false' for new instances")
			return
		}
	})
	return err
}
