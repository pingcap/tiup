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

package cluster

import (
	"fmt"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
)

// InstanceIter to iterate instance.
type InstanceIter interface {
	IterInstance(func(inst spec.Instance))
}

// BuildDownloadCompTasks build download component tasks
func BuildDownloadCompTasks(version string, instanceIter InstanceIter, bindVersion spec.BindVersion) []*task.StepDisplay {
	var tasks []*task.StepDisplay
	uniqueTaskList := make(map[string]struct{}) // map["comp-os-arch"]{}
	instanceIter.IterInstance(func(inst spec.Instance) {
		key := fmt.Sprintf("%s-%s-%s", inst.ComponentName(), inst.OS(), inst.Arch())
		if _, found := uniqueTaskList[key]; !found {
			uniqueTaskList[key] = struct{}{}

			// download spark as dependency of tispark
			if inst.ComponentName() == spec.ComponentTiSpark {
				tasks = append(tasks, buildDownloadSparkTask(version, inst, bindVersion))
			}

			version := bindVersion(inst.ComponentName(), version)
			t := task.NewBuilder().
				Download(inst.ComponentName(), inst.OS(), inst.Arch(), version).
				BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)",
					inst.ComponentName(), version, inst.OS(), inst.Arch()))
			tasks = append(tasks, t)
		}
	})
	return tasks
}

// buildDownloadSparkTask build download task for spark, which is a dependency of tispark
// FIXME: this is a hack and should be replaced by dependency handling in manifest processing
func buildDownloadSparkTask(version string, inst spec.Instance, bindVersion spec.BindVersion) *task.StepDisplay {
	ver := bindVersion(spec.ComponentSpark, version)
	return task.NewBuilder().
		Download(spec.ComponentSpark, inst.OS(), inst.Arch(), ver).
		BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)",
			spec.ComponentSpark, version, inst.OS(), inst.Arch()))
}
