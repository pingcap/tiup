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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/creasty/defaults"
	. "github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"gopkg.in/yaml.v3"
)

type ansSuite struct {
}

var _ = Suite(&ansSuite{})

func TestAnsible(t *testing.T) {
	TestingT(t)
}

func (s *ansSuite) TestMonitoredDeployDir(c *C) {
	r := strings.NewReader(`
[monitored_servers]
172.16.10.1
172.16.10.2
172.16.10.3

[all:vars]
process_supervision = systemd
	`)

	_, clsMeta, _, err := parseInventoryFile(r)
	c.Assert(err, IsNil)
	c.Assert(clsMeta.Topology.MonitoredOptions.DeployDir, Equals, "")

	r = strings.NewReader(`
[monitored_servers]
172.16.10.1
172.16.10.2
172.16.10.3

[all:vars]
deploy_dir = /data1/deploy
process_supervision = systemd
	`)

	_, clsMeta, _, err = parseInventoryFile(r)
	c.Assert(err, IsNil)
	c.Assert(clsMeta.Topology.MonitoredOptions.DeployDir, Equals, "/data1/deploy")

	r = strings.NewReader(`
[monitored_servers]
172.16.10.1 deploy_dir=/data/deploy
172.16.10.2 deploy_dir=/data/deploy
172.16.10.3 deploy_dir=/data/deploy

[all:vars]
deploy_dir = /data1/deploy
process_supervision = systemd
	`)

	_, clsMeta, _, err = parseInventoryFile(r)
	c.Assert(err, IsNil)
	c.Assert(clsMeta.Topology.MonitoredOptions.DeployDir, Equals, "/data/deploy")

	r = strings.NewReader(`
[monitored_servers]
172.16.10.1 deploy_dir=/data/deploy1
172.16.10.2 deploy_dir=/data/deploy2
172.16.10.3 deploy_dir=/data/deploy3

[all:vars]
deploy_dir = /data1/deploy
process_supervision = systemd
	`)

	_, clsMeta, _, err = parseInventoryFile(r)
	c.Assert(err, IsNil)
	c.Assert(clsMeta.Topology.MonitoredOptions.DeployDir, Equals, "/data1/deploy")
}

func (s *ansSuite) TestParseInventoryFile(c *C) {
	dir := "test-data"
	invData, err := os.Open(filepath.Join(dir, "inventory.ini"))
	c.Assert(err, IsNil)

	clsName, clsMeta, inv, err := parseInventoryFile(invData)
	c.Assert(err, IsNil)
	c.Assert(inv, NotNil)
	c.Assert(clsName, Equals, "ansible-cluster")
	c.Assert(clsMeta, NotNil)
	c.Assert(clsMeta.Version, Equals, "v3.0.12")
	c.Assert(clsMeta.User, Equals, "tiops")

	expected := []byte(`global:
    user: tiops
    deploy_dir: /home/tiopsimport/ansible-deploy
    arch: amd64
monitored:
    deploy_dir: /home/tiopsimport/ansible-deploy
    data_dir: /home/tiopsimport/ansible-deploy/data
server_configs:
    tidb:
        binlog.enable: true
    tikv: {}
    pd: {}
    tiflash: {}
    tiflash-learner: {}
    pump: {}
    drainer: {}
    cdc: {}
    grafana: {}
tidb_servers: []
tikv_servers: []
tiflash_servers: []
pd_servers: []
monitoring_servers: []
`)

	topo, err := yaml.Marshal(clsMeta.Topology)
	c.Assert(err, IsNil)
	c.Assert(topo, DeepEquals, expected)
}

func (s *ansSuite) TestParseGroupVars(c *C) {
	dir := "test-data"
	ansCfgFile := filepath.Join(dir, "ansible.cfg")
	invData, err := os.Open(filepath.Join(dir, "inventory.ini"))
	c.Assert(err, IsNil)
	_, clsMeta, inv, err := parseInventoryFile(invData)
	c.Assert(err, IsNil)

	err = parseGroupVars(context.WithValue(
		context.TODO(), logprinter.ContextKeyLogger, logprinter.NewLogger(""),
	), dir, ansCfgFile, clsMeta, inv)
	c.Assert(err, IsNil)
	err = defaults.Set(clsMeta)
	c.Assert(err, IsNil)

	var expected spec.ClusterMeta
	var metaFull spec.ClusterMeta

	expectedTopo, err := os.ReadFile(filepath.Join(dir, "meta.yaml"))
	c.Assert(err, IsNil)
	err = yaml.Unmarshal(expectedTopo, &expected)
	c.Assert(err, IsNil)

	// marshal and unmarshal the meta to ensure custom defaults are populated
	meta, err := yaml.Marshal(clsMeta)
	c.Assert(err, IsNil)
	err = yaml.Unmarshal(meta, &metaFull)
	c.Assert(err, IsNil)

	sortClusterMeta(&metaFull)
	sortClusterMeta(&expected)

	_, err = yaml.Marshal(metaFull)
	c.Assert(err, IsNil)
	c.Assert(metaFull, DeepEquals, expected)
}

func sortClusterMeta(clsMeta *spec.ClusterMeta) {
	sort.Slice(clsMeta.Topology.TiDBServers, func(i, j int) bool {
		return clsMeta.Topology.TiDBServers[i].Host < clsMeta.Topology.TiDBServers[j].Host
	})
	sort.Slice(clsMeta.Topology.TiKVServers, func(i, j int) bool {
		return clsMeta.Topology.TiKVServers[i].Host < clsMeta.Topology.TiKVServers[j].Host
	})
	sort.Slice(clsMeta.Topology.TiFlashServers, func(i, j int) bool {
		return clsMeta.Topology.TiFlashServers[i].Host < clsMeta.Topology.TiFlashServers[j].Host
	})
	sort.Slice(clsMeta.Topology.PDServers, func(i, j int) bool {
		return clsMeta.Topology.PDServers[i].Host < clsMeta.Topology.PDServers[j].Host
	})
	sort.Slice(clsMeta.Topology.PumpServers, func(i, j int) bool {
		return clsMeta.Topology.PumpServers[i].Host < clsMeta.Topology.PumpServers[j].Host
	})
	sort.Slice(clsMeta.Topology.Drainers, func(i, j int) bool {
		return clsMeta.Topology.Drainers[i].Host < clsMeta.Topology.Drainers[j].Host
	})
	sort.Slice(clsMeta.Topology.CDCServers, func(i, j int) bool {
		return clsMeta.Topology.CDCServers[i].Host < clsMeta.Topology.CDCServers[j].Host
	})
	sort.Slice(clsMeta.Topology.Monitors, func(i, j int) bool {
		return clsMeta.Topology.Monitors[i].Host < clsMeta.Topology.Monitors[j].Host
	})
	sort.Slice(clsMeta.Topology.Grafanas, func(i, j int) bool {
		return clsMeta.Topology.Grafanas[i].Host < clsMeta.Topology.Grafanas[j].Host
	})
	sort.Slice(clsMeta.Topology.Alertmanagers, func(i, j int) bool {
		return clsMeta.Topology.Alertmanagers[i].Host < clsMeta.Topology.Alertmanagers[j].Host
	})
}

func withTempFile(content string, fn func(string)) {
	file, err := os.CreateTemp("/tmp", "topology-test")
	if err != nil {
		panic(fmt.Sprintf("create temp file: %s", err))
	}
	defer os.Remove(file.Name())

	_, err = file.WriteString(content)
	if err != nil {
		panic(fmt.Sprintf("write temp file: %s", err))
	}
	file.Close()

	fn(file.Name())
}

func (s *ansSuite) TestParseConfig(c *C) {
	// base test
	withTempFile(`
a = true

[b]
c = 1
d = "\""
`, func(file string) {
		m, err := parseConfigFile(file)
		c.Assert(err, IsNil)
		c.Assert(m["x"], IsNil)
		c.Assert(m["a"], Equals, true)
		c.Assert(m["b.c"], Equals, int64(1))
		c.Assert(m["b.d"], Equals, "\"")
	})
}

func (s *ansSuite) TestDiffConfig(c *C) {
	global, locals := diffConfigs([]map[string]interface{}{
		{
			"a":       true,
			"b":       1,
			"foo.bar": 1,
		},
		{
			"a":       true,
			"b":       2,
			"foo.bar": 1,
		},
		{
			"a":       true,
			"b":       3,
			"foo.bar": 1,
		},
	})

	c.Assert(global["a"], NotNil)
	c.Assert(global["b"], IsNil)
	c.Assert(global["a"], Equals, true)
	c.Assert(global["foo.bar"], Equals, 1)
	c.Assert(locals[0]["b"], Equals, 1)
	c.Assert(locals[1]["b"], Equals, 2)
	c.Assert(locals[2]["b"], Equals, 3)
}
