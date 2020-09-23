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
	"fmt"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
)

// ImportConfig copies config files from cluster which deployed through tidb-ansible
func ImportConfig(name string, clsMeta *spec.ClusterMeta, sshTimeout int64, sshType executor.SSHType) error {
	// there may be already cluster dir, skip create
	//if err := os.MkdirAll(meta.ClusterPath(name), 0755); err != nil {
	//	return err
	//}
	//if err := ioutil.WriteFile(meta.ClusterPath(name, "topology.yaml"), yamlFile, 0664); err != nil {
	//	return err
	//}
	var copyFileTasks []task.Task
	for _, comp := range clsMeta.Topology.ComponentsByStartOrder() {
		log.Infof("Copying config file(s) of %s...", comp.Name())
		for _, inst := range comp.Instances() {
			switch inst.ComponentName() {
			case spec.ComponentPD, spec.ComponentTiKV, spec.ComponentPump, spec.ComponentTiDB, spec.ComponentDrainer:
				t := task.NewBuilder().
					SSHKeySet(
						spec.ClusterPath(name, "ssh", "id_rsa"),
						spec.ClusterPath(name, "ssh", "id_rsa.pub")).
					UserSSH(inst.GetHost(), inst.GetSSHPort(), clsMeta.User, sshTimeout, sshType, "").
					CopyFile(filepath.Join(inst.DeployDir(), "conf", inst.ComponentName()+".toml"),
						spec.ClusterPath(name,
							spec.AnsibleImportedConfigPath,
							fmt.Sprintf("%s-%s-%d.toml",
								inst.ComponentName(),
								inst.GetHost(),
								inst.GetPort())),
						inst.GetHost(),
						true).
					Build()
				copyFileTasks = append(copyFileTasks, t)
			case spec.ComponentTiFlash:
				t := task.NewBuilder().
					SSHKeySet(
						spec.ClusterPath(name, "ssh", "id_rsa"),
						spec.ClusterPath(name, "ssh", "id_rsa.pub")).
					UserSSH(inst.GetHost(), inst.GetSSHPort(), clsMeta.User, sshTimeout, sshType, "").
					CopyFile(filepath.Join(inst.DeployDir(), "conf", inst.ComponentName()+".toml"),
						spec.ClusterPath(name,
							spec.AnsibleImportedConfigPath,
							fmt.Sprintf("%s-%s-%d.toml",
								inst.ComponentName(),
								inst.GetHost(),
								inst.GetPort())),
						inst.GetHost(),
						true).
					CopyFile(filepath.Join(inst.DeployDir(), "conf", inst.ComponentName()+"-learner.toml"),
						spec.ClusterPath(name,
							spec.AnsibleImportedConfigPath,
							fmt.Sprintf("%s-learner-%s-%d.toml",
								inst.ComponentName(),
								inst.GetHost(),
								inst.GetPort())),
						inst.GetHost(),
						true).
					Build()
				copyFileTasks = append(copyFileTasks, t)
			default:
				break
			}
		}
	}
	t := task.NewBuilder().
		Parallel(false, copyFileTasks...).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		return errors.Trace(err)
	}
	log.Infof("Finished copying configs.")
	return nil
}
