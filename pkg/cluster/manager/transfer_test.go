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

package manager

import (
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/stretchr/testify/assert"
)

func TestRenderSpec(t *testing.T) {
	var s spec.Instance = &spec.TiDBInstance{BaseInstance: spec.BaseInstance{
		InstanceSpec: &spec.TiDBSpec{
			Host:       "172.16.5.140",
			SSHPort:    22,
			Imported:   false,
			Port:       4000,
			StatusPort: 10080,
			DeployDir:  "/home/test/deploy/tidb-4000",
			Arch:       "amd64",
			OS:         "linux",
		},
	}}
	dir, err := renderSpec("{{.DataDir}}", s, "test-tidb")
	assert.NotNil(t, err)
	assert.Empty(t, dir)

	s = &spec.PDInstance{BaseInstance: spec.BaseInstance{
		InstanceSpec: &spec.PDSpec{
			Host:       "172.16.5.140",
			SSHPort:    22,
			Imported:   false,
			Name:       "pd-1",
			ClientPort: 2379,
			PeerPort:   2380,
			DeployDir:  "/home/test/deploy/pd-2379",
			DataDir:    "/home/test/deploy/pd-2379/data",
		},
	}}
	// s.BaseInstance.InstanceSpec
	dir, err = renderSpec("{{.DataDir}}", s, "test-pd")
	assert.Nil(t, err)
	assert.NotEmpty(t, dir)

	s = &spec.TSOInstance{BaseInstance: spec.BaseInstance{
		InstanceSpec: &spec.TSOSpec{
			Host:      "172.16.5.140",
			SSHPort:   22,
			Name:      "tso-1",
			DeployDir: "/home/test/deploy/tso-3379",
			DataDir:   "/home/test/deploy/tso-3379/data",
		},
	}}
	// s.BaseInstance.InstanceSpec
	dir, err = renderSpec("{{.DataDir}}", s, "test-tso")
	assert.Nil(t, err)
	assert.NotEmpty(t, dir)

	s = &spec.SchedulingInstance{BaseInstance: spec.BaseInstance{
		InstanceSpec: &spec.SchedulingSpec{
			Host:      "172.16.5.140",
			SSHPort:   22,
			Name:      "scheduling-1",
			DeployDir: "/home/test/deploy/scheduling-3379",
			DataDir:   "/home/test/deploy/scheduling-3379/data",
		},
	}}
	// s.BaseInstance.InstanceSpec
	dir, err = renderSpec("{{.DataDir}}", s, "test-scheduling")
	assert.Nil(t, err)
	assert.NotEmpty(t, dir)
}
