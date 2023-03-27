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

package ansible

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/tui"
)

// ImportConfig copies config files from cluster which deployed through tidb-ansible
func ImportConfig(ctx context.Context, name string, clsMeta *spec.ClusterMeta, gOpt operator.Options) error {
	// there may be already cluster dir, skip create
	// if err := utils.MkdirAll(meta.ClusterPath(name), 0755); err != nil {
	// 	 return err
	// }
	// if err := utils.WriteFile(meta.ClusterPath(name, "topology.yaml"), yamlFile, 0664); err != nil {
	// 	 return err
	// }

	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	var sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	if gOpt.SSHType != executor.SSHTypeNone && len(gOpt.SSHProxyHost) != 0 {
		var err error
		if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
			return err
		}
	}
	var copyFileTasks []task.Task
	for _, comp := range clsMeta.Topology.ComponentsByStartOrder() {
		logger.Infof("Copying config file(s) of %s...", comp.Name())
		for _, inst := range comp.Instances() {
			switch inst.ComponentName() {
			case spec.ComponentPD, spec.ComponentTiKV, spec.ComponentPump, spec.ComponentTiDB, spec.ComponentDrainer:
				t := task.NewBuilder(logger).
					SSHKeySet(
						spec.ClusterPath(name, "ssh", "id_rsa"),
						spec.ClusterPath(name, "ssh", "id_rsa.pub")).
					UserSSH(
						inst.GetManageHost(),
						inst.GetSSHPort(),
						clsMeta.User,
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
						"",
					).
					CopyFile(filepath.Join(inst.DeployDir(), "conf", inst.ComponentName()+".toml"),
						spec.ClusterPath(name,
							spec.AnsibleImportedConfigPath,
							fmt.Sprintf("%s-%s-%d.toml",
								inst.ComponentName(),
								inst.GetHost(),
								inst.GetPort())),
						inst.GetManageHost(),
						true,
						0,
						false).
					Build()
				copyFileTasks = append(copyFileTasks, t)
			case spec.ComponentTiFlash:
				t := task.NewBuilder(logger).
					SSHKeySet(
						spec.ClusterPath(name, "ssh", "id_rsa"),
						spec.ClusterPath(name, "ssh", "id_rsa.pub")).
					UserSSH(
						inst.GetManageHost(),
						inst.GetSSHPort(),
						clsMeta.User,
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
						"",
					).
					CopyFile(filepath.Join(inst.DeployDir(), "conf", inst.ComponentName()+".toml"),
						spec.ClusterPath(name,
							spec.AnsibleImportedConfigPath,
							fmt.Sprintf("%s-%s-%d.toml",
								inst.ComponentName(),
								inst.GetHost(),
								inst.GetPort())),
						inst.GetManageHost(),
						true,
						0,
						false).
					CopyFile(filepath.Join(inst.DeployDir(), "conf", inst.ComponentName()+"-learner.toml"),
						spec.ClusterPath(name,
							spec.AnsibleImportedConfigPath,
							fmt.Sprintf("%s-learner-%s-%d.toml",
								inst.ComponentName(),
								inst.GetHost(),
								inst.GetPort())),
						inst.GetManageHost(),
						true,
						0,
						false).
					Build()
				copyFileTasks = append(copyFileTasks, t)
			}
		}
	}
	t := task.NewBuilder(logger).
		Parallel(false, copyFileTasks...).
		Build()

	if err := t.Execute(ctx); err != nil {
		return errors.Trace(err)
	}
	logger.Infof("Finished copying configs.")
	return nil
}
