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
	"path/filepath"
	"strings"

	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type scaleOutOptions struct {
	user       string // username to login to the SSH server
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
			if len(opt.keyFile) == 0 && len(opt.password) == 0 {
				return errors.New("password and key need to specify at least one")
			}
			return scaleOut(args[0], args[1], opt)
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", "root", "Specify the system user name")
	cmd.Flags().StringVar(&opt.password, "password", "", "Specify the password of system user")
	cmd.Flags().StringVar(&opt.keyFile, "key", "", "Specify the key path of system user")
	cmd.Flags().StringVar(&opt.passphrase, "passphrase", "", "Specify the passphrase of the key")

	_ = cmd.MarkFlagRequired("version")

	return cmd
}

func scaleOut(name, topoFile string, opt scaleOutOptions) error {
	var newPart meta.TopologySpecification
	if err := utils.ParseYaml(topoFile, &newPart); err != nil {
		return err
	}

	t, err := bootstrapNewPart(name, opt, &newPart)
	if err != nil {
		return err
	}
	if err := t.Execute(task.NewContext()); err != nil {
		return err
	}

	metadata, err := meta.ClusterMetadata(name)
	if err != nil {
		return err
	}
	topo := metadata.Topology.Merge(&newPart)

	t, err = refreshConfig(name, metadata, topo)
	if err != nil {
		return err
	}
	if err := t.Execute(task.NewContext()); err != nil {
		return err
	}

	metadata.Topology = topo
	return meta.SaveClusterMeta(name, metadata)
}

func bootstrapNewPart(name string, opt scaleOutOptions, newPart *meta.TopologySpecification) (task.Task, error) {
	metadata, err := meta.ClusterMetadata(name)
	if err != nil {
		return nil, err
	}
	oldPart := metadata.Topology
	var (
		envInitTasks      []task.Task // tasks which are used to initialize environment
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueHosts = set.NewStringSet()
	)
	for _, comp := range newPart.ComponentsByStartOrder() {
		for idx, inst := range comp.Instances() {
			version := getComponentVersion(inst.ComponentName(), metadata.Version)
			if version == "" {
				return nil, errors.Errorf("unsupported component: %v", inst.ComponentName())
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
					EnvInit(inst.GetHost(), metadata.User).
					Build()
				envInitTasks = append(envInitTasks, t)
			}

			deployDir := inst.DeployDir()
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/", metadata.User, deployDir)
			}
			// Deploy component
			t := task.NewBuilder().
				UserSSH(inst.GetHost(), metadata.User).
				Mkdir(inst.GetHost(),
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "conf"),
					filepath.Join(deployDir, "scripts"),
					filepath.Join(deployDir, "log")).
				CopyComponent(inst.ComponentName(), version, inst.GetHost(), deployDir).
				ScaleConfig(name, oldPart, inst, metadata.User, deployDir).
				Build()
			copyCompTasks = append(copyCompTasks, t)
		}
	}

	return task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(name, "ssh", "id_rsa"),
			meta.ClusterPath(name, "ssh", "id_rsa.pub")).
		Parallel(envInitTasks...).
		Parallel(downloadCompTasks...).
		Parallel(copyCompTasks...).
		ClusterOperate(newPart, operator.StartOperation, operator.Options{}).
		Build(), nil
}

func refreshConfig(name string, metadata *meta.ClusterMeta, topo *meta.Specification) (task.Task, error) {
	tasks := []task.Task{}
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			deployDir := inst.DeployDir()
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/", metadata.User, deployDir)
			}
			t := task.NewBuilder().
				UserSSH(inst.GetHost(), metadata.User).
				InitConfig(name, inst, metadata.User, deployDir).
				Build()
			tasks = append(tasks, t)
		}
	}
	return task.NewBuilder().
		SSHKeySet(
			meta.ClusterPath(name, "ssh", "id_rsa"),
			meta.ClusterPath(name, "ssh", "id_rsa.pub")).
		Parallel(tasks...).Build(), nil
}
