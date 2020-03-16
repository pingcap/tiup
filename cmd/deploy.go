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
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/topology"
)

func runDeploy(topo *topology.Specification) error {
	var tasks []task.Task
	for _, db := range topo.TiDBServers {
		t := task.NewBuilder().
			SSH(db.IP, "keypath").
			Parallel(
				&task.CopyFile{ /*binary*/ },
				&task.CopyFile{ /*configuration*/ },
			).
			CopyFile("x", db.IP, "xx").
			Build()
		tasks = append(tasks, t)
	}
	// PD/KVProm/Grafana...

	return task.Parallel(tasks).Execute(&task.Context{})
}
