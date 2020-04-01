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
	"github.com/pingcap-incubator/tiops/pkg/cliutil"
	"github.com/pingcap-incubator/tiops/pkg/logger"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

var errPasswordKeyAtLeastOne = errors.New("--password and --key need to specify at least one")

type scaleOutOptions struct {
	user       string // username to login to the SSH server
	usePasswd  bool   // use password for authentication
	password   string // password of the user
	keyFile    string // path to the private key file
	passphrase string // passphrase of the private key file
}

func newScaleOutCmd() *cobra.Command {
	opt := scaleOutOptions{}
	cmd := &cobra.Command{
		Use:          "scale-out <cluster-name> <topology.yaml>",
		Short:        "Scale out a TiDB cluster",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}
			if opt.usePasswd {
				opt.password = cliutil.PromptForPassword("Password: ")
				fmt.Println("")
			}
			if len(opt.keyFile) == 0 && len(opt.password) == 0 {
				return errPasswordKeyAtLeastOne
			}

			logger.EnableAuditLog()
			return scaleOut(args[0], args[1], opt)
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", "root", "Specify the system user name")
	cmd.Flags().BoolVar(&opt.usePasswd, "password", false, "Specify the password of system user")
	cmd.Flags().StringVarP(&opt.keyFile, "identity_file", "i", "", "Specify the key path of system user")
	cmd.Flags().StringVar(&opt.passphrase, "passphrase", "", "Specify the passphrase of the key")

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

	// Inherit existing global configuration
	newPart.GlobalOptions = metadata.Topology.GlobalOptions
	newPart.MonitoredOptions = metadata.Topology.MonitoredOptions
	newPart.ServerConfigs = metadata.Topology.ServerConfigs

	// Build the scale out tasks
	t, err := buildScaleOutTask(clusterName, metadata, mergedTopo, opt, &newPart)
	if err != nil {
		return err
	}

	return t.Execute(task.NewContext())
}

func buildScaleOutTask(
	clusterName string,
	metadata *meta.ClusterMeta,
	mergedTopo *meta.Specification,
	opt scaleOutOptions,
	newPart *meta.TopologySpecification) (task.Task, error) {
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
	uninitializedHosts := set.NewStringSet()
	newPart.IterInstance(func(instance meta.Instance) {
		if host := instance.GetHost(); !initializedHosts.Exist(host) {
			uninitializedHosts.Insert(host)
			t := task.NewBuilder().
				RootSSH(instance.GetHost(), instance.GetSSHPort(), opt.user, opt.password, opt.keyFile, opt.passphrase).
				EnvInit(instance.GetHost(), metadata.User).
				Build()
			envInitTasks = append(envInitTasks, t)
		}
	})

	// Download missing component
	downloadCompTasks = buildDownloadCompTasks(metadata.Version, newPart)

	// Deploy the new topology and refresh the configuration
	newPart.IterInstance(func(inst meta.Instance) {
		version := bindversion.ComponentVersion(inst.ComponentName(), metadata.Version)
		deployDir := inst.DeployDir()
		if !strings.HasPrefix(deployDir, "/") {
			deployDir = filepath.Join("/home/", metadata.User, deployDir)
		}
		// data dir would be empty for components which don't need it
		dataDir := inst.DataDir()
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join("/home/", metadata.User, dataDir)
		}
		// log dir will always be with values, but might not used by the component
		logDir := inst.LogDir()
		if !strings.HasPrefix(logDir, "/") {
			logDir = filepath.Join("/home/", metadata.User, logDir)
		}

		// Deploy component
		t := task.NewBuilder().
			UserSSH(inst.GetHost(), metadata.User).
			Mkdir(inst.GetHost(),
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts"),
				dataDir,
				logDir).
			CopyComponent(inst.ComponentName(), version, inst.GetHost(), deployDir).
			ScaleConfig(clusterName,
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
		deployDir := inst.DeployDir()
		if !strings.HasPrefix(deployDir, "/") {
			deployDir = filepath.Join("/home/", metadata.User, deployDir)
		}
		dataDir := inst.DataDir()
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join("/home/", metadata.User, dataDir)
		}
		logDir := inst.LogDir()
		if !strings.HasPrefix(logDir, "/") {
			logDir = filepath.Join("/home/", metadata.User, logDir)
		}
		// Refresh all configuration
		t := task.NewBuilder().
			UserSSH(inst.GetHost(), metadata.User).
			InitConfig(clusterName, inst, metadata.User, meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDir,
				Log:    logDir,
			}).
			Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(
		clusterName,
		uninitializedHosts,
		metadata.Topology.GlobalOptions,
		metadata.Topology.MonitoredOptions,
		metadata.Version)
	downloadCompTasks = append(downloadCompTasks, dlTasks...)
	deployCompTasks = append(deployCompTasks, dpTasks...)

	return task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(clusterName, "ssh", "id_rsa"),
			meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
		Parallel(downloadCompTasks...).
		Parallel(envInitTasks...).
		Parallel(deployCompTasks...).
		// TODO: find another way to make sure current cluster started
		ClusterSSH(metadata.Topology, metadata.User).
		ClusterOperate(metadata.Topology, operator.StartOperation, operator.Options{}).
		ClusterSSH(newPart, metadata.User).
		Func("save meta", func() error {
			metadata.Topology = mergedTopo
			return meta.SaveClusterMeta(clusterName, metadata)
		}).
		ClusterOperate(newPart, operator.StartOperation, operator.Options{}).
		Parallel(refreshConfigTasks...).
		Build(), nil

}
