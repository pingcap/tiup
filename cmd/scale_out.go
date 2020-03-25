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
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type scaleOutOptions struct {
	version    string // version of the cluster
	user       string // username to login to the SSH server
	deployUser string // username of deploy tidb
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
	cmd.Flags().StringVarP(&opt.deployUser, "deploy-user", "d", "tidb", "Specify the user name of deploy cluster")
	cmd.Flags().StringVar(&opt.password, "password", "", "Specify the password of system user")
	cmd.Flags().StringVar(&opt.keyFile, "key", "", "Specify the key path of system user")
	cmd.Flags().StringVar(&opt.passphrase, "passphrase", "", "Specify the passphrase of the key")
	cmd.Flags().StringVar(&opt.version, "version", "", "Specify the deploy version of new nodes")

	_ = cmd.MarkFlagRequired("version")

	return cmd
}

func scaleOut(name, topoFile string, opt scaleOutOptions) error {
	var oldPart meta.TopologySpecification
	var newPart meta.TopologySpecification
	if err := utils.ParseYaml(meta.ClusterPath(name, "topology.yaml"), &oldPart); err != nil {
		return err
	}
	if err := utils.ParseYaml(topoFile, &newPart); err != nil {
		return err
	}

	var (
		envInitTasks      []task.Task // tasks which are used to initialize environment
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueHosts = set.NewStringSet()
	)

	for _, comp := range newPart.ComponentsByStartOrder() {
		for idx, inst := range comp.Instances() {
			version := getComponentVersion(inst.ComponentName(), opt.version)
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
					EnvInit(inst.GetHost(), opt.deployUser).
					UserSSH(inst.GetHost(), opt.deployUser).
					Build()
				envInitTasks = append(envInitTasks, t)
			}

			deployDir := inst.DeployDir()
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/"+opt.deployUser+"/deploy", deployDir)
			}
			// Deploy component
			t := task.NewBuilder().
				Mkdir(inst.GetHost(),
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "data"),
					filepath.Join(deployDir, "config"),
					filepath.Join(deployDir, "scripts"),
					filepath.Join(deployDir, "logs")).
				CopyComponent(inst.ComponentName(), version, inst.GetHost(), deployDir).
				// ScaleConfig(name, &oldPart, inst, deployDir)
				Build()
			copyCompTasks = append(copyCompTasks, t)
		}
	}

	t := task.NewBuilder().
		SSHKeyGen(meta.ClusterPath(name, "ssh", "id_rsa")).
		Parallel(envInitTasks...).
		Parallel(downloadCompTasks...).
		Parallel(copyCompTasks...).
		Build()

	return t.Execute(task.NewContext())
}
