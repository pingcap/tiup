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

package meta

import (
	. "github.com/pingcap/check"
	"gopkg.in/yaml.v2"
	"testing"
)

type metaSuite struct {
}

var _ = Suite(&metaSuite{})

func TestMeta(t *testing.T) {
	TestingT(t)
}

func (s *metaSuite) TestGlobalOptions(c *C) {
	topo := TopologySpecification{}
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
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.User, Equals, "test1")
	c.Assert(topo.GlobalOptions.SSHPort, Equals, 220)
	c.Assert(topo.TiDBServers[0].SSHPort, Equals, 220)
	c.Assert(topo.TiDBServers[0].DeployDir, Equals, "tidb-deploy")

	c.Assert(topo.PDServers[0].SSHPort, Equals, 220)
	c.Assert(topo.PDServers[0].DeployDir, Equals, "test-deploy/pd-2379")
	c.Assert(topo.PDServers[0].DataDir, Equals, "pd-data")
}
