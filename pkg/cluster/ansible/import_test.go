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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/creasty/defaults"
	. "github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"gopkg.in/yaml.v2"
)

type ansSuite struct {
}

var _ = Suite(&ansSuite{})

func TestAnsible(t *testing.T) {
	TestingT(t)
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

	err = parseGroupVars(dir, ansCfgFile, clsMeta, inv)
	c.Assert(err, IsNil)
	err = defaults.Set(clsMeta)
	c.Assert(err, IsNil)

	var expected spec.ClusterMeta
	var metaFull spec.ClusterMeta

	expectedTopo, err := ioutil.ReadFile(filepath.Join(dir, "meta.yaml"))
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

	actual, err := yaml.Marshal(metaFull)
	c.Assert(err, IsNil)
	fmt.Printf("Got meta:\n%s\n", actual)

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
	sort.Slice(clsMeta.Topology.Grafana, func(i, j int) bool {
		return clsMeta.Topology.Grafana[i].Host < clsMeta.Topology.Grafana[j].Host
	})
	sort.Slice(clsMeta.Topology.Alertmanager, func(i, j int) bool {
		return clsMeta.Topology.Alertmanager[i].Host < clsMeta.Topology.Alertmanager[j].Host
	})
}
