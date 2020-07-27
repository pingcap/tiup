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

package spec

import (
	. "github.com/pingcap/check"
	"gopkg.in/yaml.v2"
)

type metaSuiteDM struct {
}

var _ = Suite(&metaSuiteDM{})

func (s *metaSuiteDM) TestDefaultDataDir(c *C) {
	// Test with without global DataDir.
	topo := new(Topology)
	topo.Masters = append(topo.Masters, MasterSpec{Host: "1.1.1.1", Port: 1111})
	topo.Workers = append(topo.Workers, WorkerSpec{Host: "1.1.2.1", Port: 2221})
	data, err := yaml.Marshal(topo)
	c.Assert(err, IsNil)

	// Check default value.
	topo = new(Topology)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "data")
	c.Assert(topo.Masters[0].DataDir, Equals, "data")
	c.Assert(topo.Workers[0].DataDir, Equals, "data")

	// Can keep the default value.
	data, err = yaml.Marshal(topo)
	c.Assert(err, IsNil)
	topo = new(Topology)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "data")
	c.Assert(topo.Masters[0].DataDir, Equals, "data")
	c.Assert(topo.Workers[0].DataDir, Equals, "data")

	// Test with global DataDir.
	topo = new(Topology)
	topo.GlobalOptions.DataDir = "/gloable_data"
	topo.Masters = append(topo.Masters, MasterSpec{Host: "1.1.1.1", Port: 1111})
	topo.Masters = append(topo.Masters, MasterSpec{Host: "1.1.1.2", Port: 1112, DataDir: "/my_data"})
	topo.Workers = append(topo.Workers, WorkerSpec{Host: "1.1.2.1", Port: 2221})
	topo.Workers = append(topo.Workers, WorkerSpec{Host: "1.1.2.2", Port: 2222, DataDir: "/my_data"})
	data, err = yaml.Marshal(topo)
	c.Assert(err, IsNil)

	topo = new(Topology)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "/gloable_data")
	c.Assert(topo.Masters[0].DataDir, Equals, "/gloable_data/dm-master-1111")
	c.Assert(topo.Masters[1].DataDir, Equals, "/my_data")
	c.Assert(topo.Workers[0].DataDir, Equals, "/gloable_data/dm-worker-2221")
	c.Assert(topo.Workers[1].DataDir, Equals, "/my_data")
}

func (s *metaSuiteDM) TestGlobalOptions(c *C) {
	topo := Topology{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data" 
dm-master_servers:
  - host: 172.16.5.138
    deploy_dir: "master-deploy"
dm-worker_servers:
  - host: 172.16.5.53
    data_dir: "worker-data"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.User, Equals, "test1")
	c.Assert(topo.GlobalOptions.SSHPort, Equals, 220)
	c.Assert(topo.Masters[0].SSHPort, Equals, 220)
	c.Assert(topo.Masters[0].DeployDir, Equals, "master-deploy")

	c.Assert(topo.Workers[0].SSHPort, Equals, 220)
	c.Assert(topo.Workers[0].DeployDir, Equals, "test-deploy/dm-worker-8262")
	c.Assert(topo.Workers[0].DataDir, Equals, "worker-data")
}

func (s *metaSuiteDM) TestDirectoryConflicts(c *C) {
	topo := Topology{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data" 
dm-master_servers:
  - host: 172.16.5.138
    deploy_dir: "/test-1"
dm-worker_servers:
  - host: 172.16.5.138
    data_dir: "/test-1"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/test-1' between 'dm-master_servers:172.16.5.138.deploy_dir' and 'dm-worker_servers:172.16.5.138.data_dir'")

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "/test-data" 
dm-master_servers:
  - host: 172.16.5.138
    data_dir: "test-1"
dm-worker_servers:
  - host: 172.16.5.138
    data_dir: "test-1"
`), &topo)
	c.Assert(err, IsNil)
}

func (s *metaSuiteDM) TestPortConflicts(c *C) {
	topo := Topology{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data" 
dm-master_servers:
  - host: 172.16.5.138
    peer_port: 1234
dm-worker_servers:
  - host: 172.16.5.138
    port: 1234
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "port conflict for '1234' between 'dm-master_servers:172.16.5.138.peer_port' and 'dm-worker_servers:172.16.5.138.port'")

	topo = Topology{}
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 1234
dm-master_servers:
  - host: 172.16.5.138
    port: 1234
dm-worker_servers:
  - host: 172.16.5.138
    status_port: 2345
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "port conflict for '1234' between 'dm-master_servers:172.16.5.138.port' and 'monitored:172.16.5.138.node_exporter_port'")

}

func (s *metaSuiteDM) TestPlatformConflicts(c *C) {
	// aarch64 and arm64 are equal
	topo := Topology{}
	err := yaml.Unmarshal([]byte(`
global:
  os: "linux"
  arch: "aarch64"
dm-master_servers:
  - host: 172.16.5.138
    arch: "arm64"
dm-worker_servers:
  - host: 172.16.5.138
`), &topo)
	c.Assert(err, IsNil)

	// different arch defined for the same host
	topo = Topology{}
	err = yaml.Unmarshal([]byte(`
global:
  os: "linux"
dm-master_servers:
  - host: 172.16.5.138
    arch: "aarch64"
dm-worker_servers:
  - host: 172.16.5.138
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "platform mismatch for '172.16.5.138' between 'dm-master_servers:linux/arm64' and 'dm-worker_servers:linux/amd64'")

	// different os defined for the same host
	topo = Topology{}
	err = yaml.Unmarshal([]byte(`
global:
  os: "linux"
  arch: "aarch64"
dm-master_servers:
  - host: 172.16.5.138
    os: "darwin"
dm-worker_servers:
  - host: 172.16.5.138
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "platform mismatch for '172.16.5.138' between 'dm-master_servers:darwin/arm64' and 'dm-worker_servers:linux/arm64'")

}

func (s *metaSuiteDM) TestCountDir(c *C) {
	topo := Topology{}

	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
dm-master_servers:
  - host: 172.16.5.138
    deploy_dir: "master-deploy"
    data_dir: "/test-data/data-1"
dm-worker_servers:
  - host: 172.16.5.53
    data_dir: "test-1"
`), &topo)
	c.Assert(err, IsNil)
	cnt := topo.CountDir("172.16.5.53", "test-deploy/dm-worker-8262")
	c.Assert(cnt, Equals, 3)
	cnt = topo.CountDir("172.16.5.138", "/test-data/data")
	c.Assert(cnt, Equals, 0) // should not match partial path

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "/test-deploy"
dm-master_servers:
  - host: 172.16.5.138
    deploy_dir: "master-deploy"
    data_dir: "/test-data/data-1"
dm-worker_servers:
  - host: 172.16.5.138
    data_dir: "/test-data/data-2"
`), &topo)
	c.Assert(err, IsNil)
	cnt = topo.CountDir("172.16.5.138", "/test-deploy/dm-worker-8262")
	c.Assert(cnt, Equals, 2)
	cnt = topo.CountDir("172.16.5.138", "")
	c.Assert(cnt, Equals, 2)
	cnt = topo.CountDir("172.16.5.138", "test-data")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.16.5.138", "/test-data")
	c.Assert(cnt, Equals, 2)

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "/test-deploy"
  data_dir: "/test-data"
dm-master_servers:
  - host: 172.16.5.138
    data_dir: "data-1"
dm-worker_servers:
  - host: 172.16.5.138
    data_dir: "data-2"
  - host: 172.16.5.53
`), &topo)
	c.Assert(err, IsNil)
	// if per-instance data_dir is set, the global data_dir is ignored, and if it
	// is a relative path, it will be under the instance's deploy_dir
	cnt = topo.CountDir("172.16.5.138", "/test-deploy/dm-worker-8262")
	c.Assert(cnt, Equals, 3)
	cnt = topo.CountDir("172.16.5.138", "")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.16.5.53", "/test-data")
	c.Assert(cnt, Equals, 1)
}
