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
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	. "github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"gopkg.in/yaml.v2"
)

type metaSuiteTopo struct {
}

var _ = Suite(&metaSuiteTopo{})

func TestMeta(t *testing.T) {
	TestingT(t)
}

func (s *metaSuiteTopo) TestDefaultDataDir(c *C) {
	// Test with without global DataDir.
	topo := new(Specification)
	topo.TiKVServers = append(topo.TiKVServers, &TiKVSpec{Host: "1.1.1.1", Port: 22})
	topo.CDCServers = append(topo.CDCServers, &CDCSpec{Host: "2.3.3.3", Port: 22})
	data, err := yaml.Marshal(topo)
	c.Assert(err, IsNil)

	// Check default value.
	topo = new(Specification)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "data")
	c.Assert(topo.TiKVServers[0].DataDir, Equals, "data")
	c.Assert(topo.CDCServers[0].DataDir, Equals, "data")

	// Can keep the default value.
	data, err = yaml.Marshal(topo)
	c.Assert(err, IsNil)
	topo = new(Specification)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "data")
	c.Assert(topo.TiKVServers[0].DataDir, Equals, "data")
	c.Assert(topo.CDCServers[0].DataDir, Equals, "data")

	// Test with global DataDir.
	topo = new(Specification)
	topo.GlobalOptions.DataDir = "/global_data"
	topo.TiKVServers = append(topo.TiKVServers, &TiKVSpec{Host: "1.1.1.1", Port: 22})
	topo.TiKVServers = append(topo.TiKVServers, &TiKVSpec{Host: "1.1.1.2", Port: 33, DataDir: "/my_data"})
	topo.CDCServers = append(topo.CDCServers, &CDCSpec{Host: "2.3.3.3", Port: 22})
	topo.CDCServers = append(topo.CDCServers, &CDCSpec{Host: "2.3.3.4", Port: 22, DataDir: "/cdc_data"})
	data, err = yaml.Marshal(topo)
	c.Assert(err, IsNil)

	topo = new(Specification)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "/global_data")
	c.Assert(topo.TiKVServers[0].DataDir, Equals, "/global_data/tikv-22")
	c.Assert(topo.TiKVServers[1].DataDir, Equals, "/my_data")

	c.Assert(topo.CDCServers[0].DataDir, Equals, "/global_data/cdc-22")
	c.Assert(topo.CDCServers[1].DataDir, Equals, "/cdc_data")
}

func (s *metaSuiteTopo) TestGlobalOptions(c *C) {
	topo := Specification{}
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
cdc_servers:
  - host: 172.16.5.233
    data_dir: "cdc-data"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.User, Equals, "test1")
	c.Assert(topo.GlobalOptions.SSHPort, Equals, 220)
	c.Assert(topo.TiDBServers[0].SSHPort, Equals, 220)
	c.Assert(topo.TiDBServers[0].DeployDir, Equals, "tidb-deploy")

	c.Assert(topo.PDServers[0].SSHPort, Equals, 220)
	c.Assert(topo.PDServers[0].DeployDir, Equals, "test-deploy/pd-2379")
	c.Assert(topo.PDServers[0].DataDir, Equals, "pd-data")

	c.Assert(topo.CDCServers[0].SSHPort, Equals, 220)
	c.Assert(topo.CDCServers[0].DeployDir, Equals, "test-deploy/cdc-8300")
	c.Assert(topo.CDCServers[0].DataDir, Equals, "cdc-data")
}

func (s *metaSuiteTopo) TestDataDirAbsolute(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  data_dir: "/test-data"
pd_servers:
  - host: 172.16.5.53
    data_dir: "pd-data"
  - host: 172.16.5.54
    client_port: 12379
cdc_servers:
  - host: 172.16.5.233
    data_dir: "cdc-data"
  - host: 172.16.5.234
    port: 23333
`), &topo)
	c.Assert(err, IsNil)

	c.Assert(topo.PDServers[0].DataDir, Equals, "pd-data")
	c.Assert(topo.PDServers[1].DataDir, Equals, "/test-data/pd-12379")

	c.Assert(topo.CDCServers[0].DataDir, Equals, "cdc-data")
	c.Assert(topo.CDCServers[1].DataDir, Equals, "/test-data/cdc-23333")
}

func (s *metaSuiteTopo) TestGlobalConfig(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"

server_configs:
  tidb:
    status.address: 10
    port: 1230
    latch.capacity: 20480
    log.file.rotate: "123445.xxx"
  tikv:
    status.address: 10
    port: 1230
    latch.capacity: 20480
  pd:
    status.address: 10
    port: 1230
    scheduler.max_limit: 20480

tidb_servers:
  - host: 172.16.5.138
    port: 1234
    config:
      latch.capacity: 3000
      log.file.rotate: "44444.xxx"
  - host: 172.16.5.139
    port: 1234
    config:
      latch.capacity: 5000
      log.file.rotate: "55555.xxx"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.ServerConfigs.TiDB, DeepEquals, map[string]interface{}{
		"status.address":  10,
		"port":            1230,
		"latch.capacity":  20480,
		"log.file.rotate": "123445.xxx",
	})
	expected := map[string]interface{}{
		"status": map[string]interface{}{
			"address": 10,
		},
		"port": 1230,
		"latch": map[string]interface{}{
			"capacity": 20480,
		},
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"rotate": "123445.xxx",
			},
		},
	}
	got := FoldMap(topo.ServerConfigs.TiDB)
	c.Assert(got, DeepEquals, expected)
	buf := &bytes.Buffer{}
	err = toml.NewEncoder(buf).Encode(expected)
	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, `port = 1230

[latch]
  capacity = 20480

[log]
  [log.file]
    rotate = "123445.xxx"

[status]
  address = 10
`)

	expected = map[string]interface{}{
		"latch": map[string]interface{}{
			"capacity": 3000,
		},
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"rotate": "44444.xxx",
			},
		},
	}
	got = FoldMap(topo.TiDBServers[0].Config)
	c.Assert(err, IsNil)
	c.Assert(got, DeepEquals, expected)

	expected = map[string]interface{}{
		"latch": map[string]interface{}{
			"capacity": 5000,
		},
		"log": map[string]interface{}{
			"file": map[string]interface{}{
				"rotate": "55555.xxx",
			},
		},
	}
	got = FoldMap(topo.TiDBServers[1].Config)
	c.Assert(err, IsNil)
	c.Assert(got, DeepEquals, expected)
}

func (s *metaSuiteTopo) TestGlobalConfigPatch(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
tikv_sata_config: &tikv_sata_config
  config.item1: 100
  config.item2: 300
  config.item3.item5: 500
  config.item3.item6: 600

tikv_servers:
  - host: 172.16.5.138
    config: *tikv_sata_config

`), &topo)
	c.Assert(err, IsNil)
	expected := map[string]interface{}{
		"config": map[string]interface{}{
			"item1": 100,
			"item2": 300,
			"item3": map[string]interface{}{
				"item5": 500,
				"item6": 600,
			},
		},
	}
	got := FoldMap(topo.TiKVServers[0].Config)
	c.Assert(err, IsNil)
	c.Assert(got, DeepEquals, expected)
}

func (s *metaSuiteTopo) TestLogDir(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
tidb_servers:
  - host: 172.16.5.138
    deploy_dir: "test-deploy"
    log_dir: "test-deploy/log"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.TiDBServers[0].LogDir, Equals, "test-deploy/log")
}

func (s *metaSuiteTopo) TestMonitorLogDir(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
monitored:
    deploy_dir: "test-deploy"
    log_dir: "test-deploy/log"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.MonitoredOptions.LogDir, Equals, "test-deploy/log")

	out, err := yaml.Marshal(topo)
	c.Assert(err, IsNil)
	err = yaml.Unmarshal(out, &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.MonitoredOptions.LogDir, Equals, "test-deploy/log")
}

func (s *metaSuiteTopo) TestMerge2Toml(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
server_configs:
  tikv:
    config.item1: 100
    config.item2: 300
    config.item3.item5: 500
    config.item3.item6: 600

tikv_servers:
  - host: 172.16.5.138
    config:
      config.item2: 500
      config.item3.item5: 700

`), &topo)
	c.Assert(err, IsNil)
	expected := `# WARNING: This file is auto-generated. Do not edit! All your modification will be overwritten!
# You can use 'tiup cluster edit-config' and 'tiup cluster reload' to update the configuration
# All configuration items you want to change can be added to:
# server_configs:
#   tikv:
#     aa.b1.c3: value
#     aa.b2.c4: value
[config]
item1 = 100
item2 = 500
[config.item3]
item5 = 700
item6 = 600
`
	got, err := merge2Toml("tikv", topo.ServerConfigs.TiKV, topo.TiKVServers[0].Config)
	c.Assert(err, IsNil)
	c.Assert(string(got), DeepEquals, expected)
}

func (s *metaSuiteTopo) TestMerge2Toml2(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: test4

monitored:
  node_exporter_port: 9100
  blackbox_exporter_port: 9110

server_configs:
  tidb:
    repair-mode: true
    log.level: debug
    log.slow-query-file: tidb-slow.log
    log.file.filename: tidb-test.log
  tikv:
    readpool.storage.use-unified-pool: true
    readpool.storage.low-concurrency: 8
  pd:
    schedule.max-merge-region-size: 20
    schedule.max-merge-region-keys: 200000
    schedule.split-merge-interval: 1h
    schedule.max-snapshot-count: 3
    schedule.max-pending-peer-count: 16
    schedule.max-store-down-time: 30m
    schedule.leader-schedule-limit: 4
    schedule.region-schedule-limit: 2048
    schedule.replica-schedule-limit: 64
    schedule.merge-schedule-limit: 8
    schedule.hot-region-schedule-limit: 4
    label-property:
      reject-leader:
        - key: "zone"
          value: "cn1"
        - key: "zone"
          value: "cn1"

tidb_servers:
  - host: 172.19.0.101

pd_servers:
  - host: 172.19.0.102
  - host: 172.19.0.104
    config:
      schedule.replica-schedule-limit: 164
      schedule.merge-schedule-limit: 18
      schedule.hot-region-schedule-limit: 14
  - host: 172.19.0.105

tikv_servers:
  - host: 172.19.0.103
`), &topo)
	c.Assert(err, IsNil)
	expected := `# WARNING: This file is auto-generated. Do not edit! All your modification will be overwritten!
# You can use 'tiup cluster edit-config' and 'tiup cluster reload' to update the configuration
# All configuration items you want to change can be added to:
# server_configs:
#   pd:
#     aa.b1.c3: value
#     aa.b2.c4: value
[label-property]

[[label-property.reject-leader]]
key = "zone"
value = "cn1"

[[label-property.reject-leader]]
key = "zone"
value = "cn1"

[schedule]
hot-region-schedule-limit = 14
leader-schedule-limit = 4
max-merge-region-keys = 200000
max-merge-region-size = 20
max-pending-peer-count = 16
max-snapshot-count = 3
max-store-down-time = "30m"
merge-schedule-limit = 18
region-schedule-limit = 2048
replica-schedule-limit = 164
split-merge-interval = "1h"
`
	got, err := merge2Toml("pd", topo.ServerConfigs.PD, topo.PDServers[1].Config)
	c.Assert(err, IsNil)
	c.Assert(string(got), DeepEquals, expected)
}

func (s *metaSuiteTopo) TestMergeImported(c *C) {
	spec := Specification{}

	// values set in topology specification of the cluster
	err := yaml.Unmarshal([]byte(`
server_configs:
  tikv:
    config.item1: 100
    config.item2: 300
    config.item3.item5: 500
    config.item3.item6: 600
    config2.item4.item7: 700

tikv_servers:
  - host: 172.16.5.138
    config:
      config.item2: 500
      config.item3.item5: 700
      config2.itemy: 1000

`), &spec)
	c.Assert(err, IsNil)

	// values set in imported configs, this will be overritten by values from
	// topology specification if present there
	config := []byte(`
[config]
item2 = 501
[config.item3]
item5 = 701
item6 = 600

[config2]
itemx = "valuex"
itemy = 999
[config2.item4]
item7 = 780
`)

	expected := `# WARNING: This file is auto-generated. Do not edit! All your modification will be overwritten!
# You can use 'tiup cluster edit-config' and 'tiup cluster reload' to update the configuration
# All configuration items you want to change can be added to:
# server_configs:
#   tikv:
#     aa.b1.c3: value
#     aa.b2.c4: value
[config]
item1 = 100
item2 = 500
[config.item3]
item5 = 700
item6 = 600

[config2]
itemx = "valuex"
itemy = 1000
[config2.item4]
item7 = 700
`

	merge1, err := mergeImported(config, spec.ServerConfigs.TiKV)
	c.Assert(err, IsNil)

	merge2, err := merge2Toml(ComponentTiKV, merge1, spec.TiKVServers[0].Config)
	c.Assert(err, IsNil)
	c.Assert(string(merge2), DeepEquals, expected)
}

func (s *metaSuiteTopo) TestTiKVLabels(c *C) {
	spec := Specification{}
	err := yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.138
    config:
      server.labels:
        dc: dc1
        zone: zone1
        host: host1
`), &spec)
	c.Assert(err, IsNil)
	labels, err := spec.TiKVServers[0].Labels()
	c.Assert(err, IsNil)
	c.Assert(labels, DeepEquals, map[string]string{
		"dc":   "dc1",
		"zone": "zone1",
		"host": "host1",
	})

	spec = Specification{}
	err = yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.138
    config:
      server.labels.dc: dc1
      server.labels.zone: zone1
      server.labels.host: host1
`), &spec)
	c.Assert(err, IsNil)
	/*
		labels, err = spec.TiKVServers[0].Labels()
		c.Assert(err, IsNil)
		c.Assert(labels, DeepEquals, map[string]string{
			"dc":   "dc1",
			"zone": "zone1",
			"host": "host1",
		})
	*/
}

func (s *metaSuiteTopo) TestLocationLabels(c *C) {
	spec := Specification{}

	lbs, err := spec.LocationLabels()
	c.Assert(err, IsNil)
	c.Assert(len(lbs), Equals, 0)

	err = yaml.Unmarshal([]byte(`
server_configs:
  pd:
    replication.location-labels: ["zone", "host"]
`), &spec)
	c.Assert(err, IsNil)
	lbs, err = spec.LocationLabels()
	c.Assert(err, IsNil)
	c.Assert(lbs, DeepEquals, []string{"zone", "host"})

	spec = Specification{}
	err = yaml.Unmarshal([]byte(`
server_configs:
  pd:
    replication:
      location-labels:
        - zone
        - host
`), &spec)
	c.Assert(err, IsNil)
	lbs, err = spec.LocationLabels()
	c.Assert(err, IsNil)
	c.Assert(lbs, DeepEquals, []string{"zone", "host"})

	spec = Specification{}
	err = yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.140
    config:
      replication:
        location-labels:
          - zone
          - host
`), &spec)
	c.Assert(err, IsNil)
	_, err = spec.LocationLabels()
	c.Assert(err, NotNil)
}

func (s *metaSuiteTopo) TestTiFlashStorageSection(c *C) {
	ctx := context.Background()
	spec := &Specification{}
	err := yaml.Unmarshal([]byte(`
tiflash_servers:
  - host: 172.16.5.138
    data_dir: /hdd0/tiflash,/hdd1/tiflash
    config:
      storage.main.dir: [/ssd0/tiflash, /ssd1/tiflash]
      storage.latest.dir: [/ssd0/tiflash]
`), spec)
	c.Assert(err, IsNil)

	flashComp := FindComponent(spec, ComponentTiFlash)
	instances := flashComp.Instances()
	c.Assert(len(instances), Equals, 1)
	// parse using clusterVersion<"v4.0.9"
	{
		ins := instances[0]
		// This should be the same with tiflash_server instance's "data_dir"
		dataDir := "/hdd0/tiflash,/hdd1/tiflash"
		cfg := scripts.NewTiFlashScript(ins.GetHost(), "", dataDir, "", "", "")
		conf, err := ins.(*TiFlashInstance).initTiFlashConfig(ctx, cfg, "v4.0.8", spec.ServerConfigs.TiFlash, meta.DirPaths{})
		c.Assert(err, IsNil)

		path, ok := conf["path"]
		c.Assert(ok, IsTrue)
		c.Assert(path, Equals, dataDir)
	}
	// parse using clusterVersion>="v4.0.9"
	checkWithVersion := func(ver string) {
		ins := instances[0].(*TiFlashInstance)
		dataDir := "/ssd0/tiflash"
		cfg := scripts.NewTiFlashScript(ins.GetHost(), "", dataDir, "", "", "")
		conf, err := ins.initTiFlashConfig(ctx, cfg, ver, spec.ServerConfigs.TiFlash, meta.DirPaths{})
		c.Assert(err, IsNil)

		_, ok := conf["path"]
		c.Assert(ok, IsTrue)

		// After merging instance configurations with "storgae", the "path" property should be removed.
		conf, err = ins.mergeTiFlashInstanceConfig(ver, conf, ins.InstanceSpec.(*TiFlashSpec).Config)
		c.Assert(err, IsNil)
		_, ok = conf["path"]
		c.Assert(ok, IsFalse)

		if storageSection, ok := conf["storage"]; ok {
			if mainSection, ok := storageSection.(map[string]interface{})["main"]; ok {
				if mainDirsSection, ok := mainSection.(map[string]interface{})["dir"]; ok {
					var mainDirs []interface{} = mainDirsSection.([]interface{})
					c.Assert(len(mainDirs), Equals, 2)
					c.Assert(mainDirs[0].(string), Equals, "/ssd0/tiflash")
					c.Assert(mainDirs[1].(string), Equals, "/ssd1/tiflash")
				} else {
					c.Error("Can not get storage.main.dir section")
				}
			} else {
				c.Error("Can not get storage.main section")
			}
			if latestSection, ok := storageSection.(map[string]interface{})["latest"]; ok {
				if latestDirsSection, ok := latestSection.(map[string]interface{})["dir"]; ok {
					var latestDirs []interface{} = latestDirsSection.([]interface{})
					c.Assert(len(latestDirs), Equals, 1)
					c.Assert(latestDirs[0].(string), Equals, "/ssd0/tiflash")
				} else {
					c.Error("Can not get storage.main.dir section")
				}
			} else {
				c.Error("Can not get storage.main section")
			}
		} else {
			c.Error("Can not get storage section")
		}
	}
	checkWithVersion("v4.0.9")
	checkWithVersion("nightly")
}

func (s *metaSuiteTopo) TestTiFlashInvalidStorageSection(c *C) {
	spec := &Specification{}

	testCases := [][]byte{
		[]byte(`
tiflash_servers:
  - host: 172.16.5.138
    data_dir: /hdd0/tiflash,/hdd1/tiflash
    config:
      # storage.main.dir is not defined
      storage.latest.dir: ["/ssd0/tiflash"]
`),
		[]byte(`
tiflash_servers:
  - host: 172.16.5.138
    data_dir: /hdd0/tiflash,/hdd1/tiflash
    config:
      # storage.main.dir is empty string array
      storage.main.dir: []
      storage.latest.dir: ["/ssd0/tiflash"]
`),
		[]byte(`
tiflash_servers:
  - host: 172.16.5.138
    data_dir: /hdd0/tiflash,/hdd1/tiflash
    config:
      # storage.main.dir is not a string array
      storage.main.dir: /hdd0/tiflash,/hdd1/tiflash
      storage.latest.dir: ["/ssd0/tiflash"]
`),
		[]byte(`
tiflash_servers:
  - host: 172.16.5.138
    data_dir: /hdd0/tiflash,/hdd1/tiflash
    config:
      # storage.main.dir is not a string array
      storage.main.dir: [0, 1]
      storage.latest.dir: ["/ssd0/tiflash"]
`),
	}

	for _, testCase := range testCases {
		err := yaml.Unmarshal(testCase, spec)
		c.Check(err, NotNil)
	}
}

func (s *metaSuiteTopo) TestTiCDCDataDir(c *C) {
	spec := &Specification{}
	err := yaml.Unmarshal([]byte(`
cdc_servers:
  - host: 172.16.6.191
    data_dir: /tidb-data/cdc-8300
`), spec)
	c.Assert(err, IsNil)

	cdcComp := FindComponent(spec, ComponentCDC)
	instances := cdcComp.Instances()
	c.Assert(len(instances), Equals, 1)

	var expected = map[string]struct {
		configSupported  bool
		dataDir          bool // data-dir is set
		dataDirSupported bool
	}{
		"v4.0.12": {false, false, false},
		"v4.0.13": {true, true, false},
		"v4.0.14": {true, true, true},

		"v5.0.0": {true, true, false},
		"v5.0.1": {true, true, false},
		"v5.0.2": {true, true, false},

		"v5.0.3": {true, true, true},
		"v5.1.0": {true, true, true},

		"v5.0.0-rc":    {false, false, false},
		"v5.1.0-alpha": {true, true, false},
		"v5.2.0-alpha": {true, true, false},
		"v6.0.0-alpha": {true, true, true},
		"v6.1.0":       {true, true, true},
		"v99.0.0":      {true, true, true},
	}

	checkByVersion := func(version string) {
		ins := instances[0].(*CDCInstance)
		cfg := scripts.NewCDCScript(ins.GetHost(), "", "", false, 0, "").
			PatchByVersion(version, ins.DataDir())

		wanted := expected[version]

		c.Assert(cfg.ConfigFileEnabled, Equals, wanted.configSupported, Commentf(version))
		c.Assert(cfg.DataDirEnabled, Equals, wanted.dataDirSupported, Commentf(version))
		c.Assert(len(cfg.DataDir) != 0, Equals, wanted.dataDir, Commentf(version))
	}

	for k := range expected {
		checkByVersion(k)
	}
}

func (s *metaSuiteTopo) TestTiFlashUsersSettings(c *C) {
	spec := &Specification{}
	err := yaml.Unmarshal([]byte(`
tiflash_servers:
  - host: 172.16.5.138
    data_dir: /ssd0/tiflash
`), spec)
	c.Assert(err, IsNil)

	ctx := context.Background()

	flashComp := FindComponent(spec, ComponentTiFlash)
	instances := flashComp.Instances()
	c.Assert(len(instances), Equals, 1)

	// parse using clusterVersion<"v4.0.12" || == "5.0.0-rc"
	checkBackwardCompatibility := func(ver string) {
		ins := instances[0].(*TiFlashInstance)
		dataDir := "/ssd0/tiflash"
		cfg := scripts.NewTiFlashScript(ins.GetHost(), "", dataDir, "", "", "")
		conf, err := ins.initTiFlashConfig(ctx, cfg, ver, spec.ServerConfigs.TiFlash, meta.DirPaths{})
		c.Assert(err, IsNil)

		// We need an empty string for 'users.default.password' for backward compatibility. Or the TiFlash process will fail to start with older versions
		if usersSection, ok := conf["users"]; !ok {
			c.Error("Can not get users section")
		} else {
			if defaultUser, ok := usersSection.(map[string]interface{})["default"]; !ok {
				c.Error("Can not get default users section")
			} else {
				var password = defaultUser.(map[string]interface{})["password"]
				c.Assert(password.(string), Equals, "")
			}
		}
	}
	checkBackwardCompatibility("v4.0.11")
	checkBackwardCompatibility("v5.0.0-rc")

	// parse using clusterVersion>="v4.0.12"
	checkWithVersion := func(ver string) {
		ins := instances[0].(*TiFlashInstance)
		dataDir := "/ssd0/tiflash"
		cfg := scripts.NewTiFlashScript(ins.GetHost(), "", dataDir, "", "", "")
		conf, err := ins.initTiFlashConfig(ctx, cfg, ver, spec.ServerConfigs.TiFlash, meta.DirPaths{})
		c.Assert(err, IsNil)

		// Those deprecated settings are ignored in newer versions
		_, ok := conf["users"]
		c.Assert(ok, IsFalse)
	}
	checkWithVersion("v4.0.12")
	checkWithVersion("v5.0.0")
	checkWithVersion("nightly")
}

func (s *metaSuiteTopo) TestYAMLAnchor(c *C) {
	topo := Specification{}
	err := yaml.UnmarshalStrict([]byte(`
global:
  custom:
    tidb_spec: &tidb_spec
      deploy_dir: "test-deploy"
      log_dir: "test-deploy/log"

tidb_servers:
  - <<: *tidb_spec
    host: 172.16.5.138
    deploy_dir: "fake-deploy"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.TiDBServers[0].Host, Equals, "172.16.5.138")
	c.Assert(topo.TiDBServers[0].DeployDir, Equals, "fake-deploy")
	c.Assert(topo.TiDBServers[0].LogDir, Equals, "test-deploy/log")
}

func (s *metaSuiteTopo) TestYAMLAnchorWithUndeclared(c *C) {
	topo := Specification{}
	err := yaml.UnmarshalStrict([]byte(`
global:
  custom:
    tidb_spec: &tidb_spec
      deploy_dir: "test-deploy"
      log_dir: "test-deploy/log"
      undeclared: "some stuff"

tidb_servers:
  - <<: *tidb_spec
    host: 172.16.5.138
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "not found"), IsTrue)
}
