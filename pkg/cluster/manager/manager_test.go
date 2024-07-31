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
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestVersionCompare(t *testing.T) {
	var err error

	err = versionCompare("v4.0.0", "v4.0.1")
	assert.Nil(t, err)

	err = versionCompare("v4.0.1", "v4.0.0")
	assert.NotNil(t, err)

	err = versionCompare("v4.0.0", "nightly")
	assert.Nil(t, err)

	err = versionCompare("nightly", "nightly")
	assert.Nil(t, err)
}

func TestValidateNewTopo(t *testing.T) {
	topo := spec.Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tidb_servers:
  - host: 172.16.5.138
    deploy_dir: "tidb-deploy"
pd_servers:
  - host: 172.16.5.53
    data_dir: "pd-data"
`), &topo)
	assert := require.New(t)
	assert.Nil(err)
	err = validateNewTopo(&topo)
	assert.Nil(err)

	topo = spec.Specification{}
	err = yaml.Unmarshal([]byte(`
tidb_servers:
  - host: 172.16.5.138
    imported: true
    deploy_dir: "tidb-deploy"
pd_servers:
  - host: 172.16.5.53
    data_dir: "pd-data"
`), &topo)
	assert.Nil(err)
	err = validateNewTopo(&topo)
	assert.NotNil(err)

	topo = spec.Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  user: "test3"
  deploy_dir: "test-deploy"
  data_dir: "test-data"
pd_servers:
  - host: 172.16.5.53
    imported: true
`), &topo)
	assert.Nil(err)
	err = validateNewTopo(&topo)
	assert.NotNil(err)
}

func TestDeduplicateCheckResult(t *testing.T) {
	checkResults := []HostCheckResult{}

	for i := 0; i <= 10; i++ {
		checkResults = append(checkResults,
			HostCheckResult{
				Node:    "127.0.0.1",
				Status:  "Warn",
				Name:    "disk",
				Message: "mount point /home does not have 'noatime' option set",
			},
		)
	}

	checkResults = deduplicateCheckResult(checkResults)

	if len(checkResults) != 1 {
		t.Errorf("Deduplicate Check Result Failed")
	}
}
