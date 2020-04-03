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

	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap/errors"
)

// ImportConfig copies config files from cluster which deployed through tidb-ansible
func ImportConfig(name string, clsMeta *meta.ClusterMeta) error {
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
			case meta.ComponentPD, meta.ComponentTiKV, meta.ComponentPump, meta.ComponentTiDB, meta.ComponentDrainer:
				t := task.NewBuilder().
					SSHKeySet(
						meta.ClusterPath(name, "ssh", "id_rsa"),
						meta.ClusterPath(name, "ssh", "id_rsa.pub")).
					UserSSH(inst.GetHost(), clsMeta.User).
					CopyFile(filepath.Join(inst.DeployDir(), "conf", inst.ComponentName()+".toml"),
						meta.ClusterPath(name,
							"config",
							fmt.Sprintf("%s-%s-%d.toml",
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
		Parallel(copyFileTasks...).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		return errors.Trace(err)
	}
	log.Infof("Finished copying configs.")
	return nil
}
