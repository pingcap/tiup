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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type componentInfo struct {
	component string
	version   repository.Version
}

type deployOptions struct {
	user       string // username to login to the SSH server
	password   string // password of the user
	keyFile    string // path to the private key file
	passphrase string // passphrase of the private key file
}

func newDeploy() *cobra.Command {
	opt := deployOptions{}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <version> <topology.yaml>",
		Short:        "Deploy a cluster for production",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 3 {
				return cmd.Help()
			}
			if len(opt.keyFile) == 0 && len(opt.password) == 0 {
				return errors.New("password and key need to specify at least one")
			}
			return deploy(args[0], args[1], args[2], opt)
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", "root", "Specify the system user name")
	cmd.Flags().StringVar(&opt.password, "password", "", "Specify the password of system user")
	cmd.Flags().StringVar(&opt.keyFile, "key", "", "Specify the key path of system user")
	cmd.Flags().StringVar(&opt.passphrase, "passphrase", "", "Specify the passphrase of the key")

	return cmd
}

// getComponentVersion maps the TiDB version to the third components binding version
func getComponentVersion(comp, version string) repository.Version {
	switch comp {
	case meta.ComponentPrometheus:
		return "v2.16.0"
	case meta.ComponentGrafana:
		return "v6.7.1"
	case meta.ComponentAlertManager:
		return "v0.20.0"
	case meta.ComponentBlackboxExporter:
		return "v0.16.0"
	case meta.ComponentNodeExporter:
		return "v0.18.1"
	case meta.ComponentPushwaygate:
		return "v1.2.0"
	default:
		return repository.Version(version)
	}
}

func deploy(name, version, topoFile string, opt deployOptions) error {
	if utils.IsExist(meta.ClusterPath(name)) {
		return errors.Errorf("cluster name '%s' exists, please choose another cluster name", name)
	}

	var topo meta.TopologySpecification
	yamlFile, err := ioutil.ReadFile(topoFile)
	if err != nil {
		return errors.Trace(err)
	}
	if err = yaml.Unmarshal(yamlFile, &topo); err != nil {
		return errors.Trace(err)
	}
	if err := os.MkdirAll(meta.ClusterPath(name), 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(meta.ClusterPath(name, "topology.yaml"), yamlFile, 0664); err != nil {
		return err
	}

	var (
		envInitTasks      []task.Task // tasks which are used to initialize environment
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueHosts = set.NewStringSet()
	)

	// topo.NormalizeDeployDir("/home/" + topo.GlobalOptions.User + "/deploy")
	for _, comp := range topo.ComponentsByStartOrder() {
		for idx, inst := range comp.Instances() {
			version := getComponentVersion(inst.ComponentName(), version)
			if version == "" {
				return errors.Errorf("unsupported component: %v", inst.ComponentName())
			}

			// Download component from repository
			if idx == 0 {
				t := task.NewBuilder().
					Download(inst.ComponentName(), version).
					Build()
				downloadCompTasks = append(downloadCompTasks, t)
			}

			// Initialize environment
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				t := task.NewBuilder().
					RootSSH(inst.GetHost(), inst.GetSSHPort(), opt.user, opt.password, opt.keyFile, opt.passphrase).
					EnvInit(inst.GetHost(), topo.GlobalOptions.User).
					UserSSH(inst.GetHost(), topo.GlobalOptions.User).
					Build()
				envInitTasks = append(envInitTasks, t)
			}

			deployDir := inst.DeployDir()
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/", topo.GlobalOptions.User, deployDir)
			}
			// Deploy component
			t := task.NewBuilder().
				Mkdir(inst.GetHost(),
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "conf"),
					filepath.Join(deployDir, "scripts"),
					filepath.Join(deployDir, "log")).
				CopyComponent(inst.ComponentName(), version, inst.GetHost(), deployDir).
				InitConfig(name, inst, topo.GlobalOptions.User, deployDir).
				Build()
			copyCompTasks = append(copyCompTasks, t)
		}
	}

	var monitoredCompTasks []task.Task
	for _, comp := range []string{meta.ComponentNodeExporter, meta.ComponentBlackboxExporter} {
		version := getComponentVersion(comp, version)
		if version == "" {
			return errors.Errorf("unsupported component: %v", comp)
		}

		t := task.NewBuilder().
			Download(comp, version).
			Build()
		downloadCompTasks = append(downloadCompTasks, t)

		for host := range uniqueHosts {
			deployDir := topo.MonitoredOptions.DeployDir
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/", topo.GlobalOptions.User, deployDir)
			}

			// Deploy component
			t := task.NewBuilder().
				Mkdir(host,
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "conf"),
					filepath.Join(deployDir, "scripts"),
					filepath.Join(deployDir, "log")).
				CopyComponent(comp, version, host, deployDir).
				MonitoredConfig(name, topo.MonitoredOptions, topo.GlobalOptions.User, deployDir).
				Build()
			monitoredCompTasks = append(monitoredCompTasks, t)
		}
	}

	t := task.NewBuilder().
		SSHKeyGen(meta.ClusterPath(name, "ssh", "id_rsa")).
		Parallel(envInitTasks...).
		Parallel(downloadCompTasks...).
		Parallel(copyCompTasks...).
		Parallel(monitoredCompTasks...).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		return errors.Trace(err)
	}

	return meta.SaveClusterMeta(name, &meta.ClusterMeta{
		User:     topo.GlobalOptions.User,
		Version:  version,
		Topology: &topo,
	})
}
