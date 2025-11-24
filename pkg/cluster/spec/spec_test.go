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
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDefaultDataDir(t *testing.T) {
	// Test with without global DataDir.
	topo := new(Specification)
	topo.TiKVServers = append(topo.TiKVServers, &TiKVSpec{Host: "1.1.1.1", Port: 22})
	topo.CDCServers = append(topo.CDCServers, &CDCSpec{Host: "2.3.3.3", Port: 22})
	topo.TiKVCDCServers = append(topo.TiKVCDCServers, &TiKVCDCSpec{Host: "3.3.3.3", Port: 22})
	topo.TiCIMetaServers = append(topo.TiCIMetaServers, &TiCIMetaSpec{Host: "4.4.4.4", Port: 22})
	topo.TiCIWorkerServers = append(topo.TiCIWorkerServers, &TiCIWorkerSpec{Host: "5.5.5.5", Port: 22})
	data, err := yaml.Marshal(topo)
	require.NoError(t, err)

	// Check default value.
	topo = new(Specification)
	err = yaml.Unmarshal(data, topo)
	require.NoError(t, err)
	require.Equal(t, "data", topo.GlobalOptions.DataDir)
	require.Equal(t, "data", topo.TiKVServers[0].DataDir)
	require.Equal(t, "data", topo.CDCServers[0].DataDir)
	require.Equal(t, "data", topo.TiKVCDCServers[0].DataDir)
	require.Equal(t, "data", topo.TiCIWorkerServers[0].DataDir)

	// Can keep the default value.
	data, err = yaml.Marshal(topo)
	require.NoError(t, err)
	topo = new(Specification)
	err = yaml.Unmarshal(data, topo)
	require.NoError(t, err)
	require.Equal(t, "data", topo.GlobalOptions.DataDir)
	require.Equal(t, "data", topo.TiKVServers[0].DataDir)
	require.Equal(t, "data", topo.CDCServers[0].DataDir)
	require.Equal(t, "data", topo.TiKVCDCServers[0].DataDir)
	require.Equal(t, "data", topo.TiCIWorkerServers[0].DataDir)

	// Test with global DataDir.
	topo = new(Specification)
	topo.GlobalOptions.DataDir = "/global_data"
	topo.TiKVServers = append(topo.TiKVServers, &TiKVSpec{Host: "1.1.1.1", Port: 22})
	topo.TiKVServers = append(topo.TiKVServers, &TiKVSpec{Host: "1.1.1.2", Port: 33, DataDir: "/my_data"})
	topo.CDCServers = append(topo.CDCServers, &CDCSpec{Host: "2.3.3.3", Port: 22})
	topo.CDCServers = append(topo.CDCServers, &CDCSpec{Host: "2.3.3.4", Port: 22, DataDir: "/cdc_data"})
	topo.TiKVCDCServers = append(topo.TiKVCDCServers, &TiKVCDCSpec{Host: "3.3.3.3", Port: 22})
	topo.TiKVCDCServers = append(topo.TiKVCDCServers, &TiKVCDCSpec{Host: "3.3.3.4", Port: 22, DataDir: "/tikv-cdc_data"})
	topo.TiCIWorkerServers = append(topo.TiCIWorkerServers, &TiCIWorkerSpec{Host: "5.5.5.5", Port: 22, DataDir: "/tici-worker-data"})
	data, err = yaml.Marshal(topo)
	require.NoError(t, err)

	topo = new(Specification)
	err = yaml.Unmarshal(data, topo)
	require.NoError(t, err)
	require.Equal(t, "/global_data", topo.GlobalOptions.DataDir)
	require.Equal(t, "/global_data/tikv-22", topo.TiKVServers[0].DataDir)
	require.Equal(t, "/my_data", topo.TiKVServers[1].DataDir)

	require.Equal(t, "/global_data/cdc-22", topo.CDCServers[0].DataDir)
	require.Equal(t, "/cdc_data", topo.CDCServers[1].DataDir)

	require.Equal(t, "/global_data/tikv-cdc-22", topo.TiKVCDCServers[0].DataDir)
	require.Equal(t, "/tikv-cdc_data", topo.TiKVCDCServers[1].DataDir)
	require.Equal(t, "/tici-worker-data", topo.TiCIWorkerServers[0].DataDir)
}

func TestGlobalOptions(t *testing.T) {
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
kvcdc_servers:
  - host: 172.16.5.244
    data_dir: "tikv-cdc-data"
tici_meta_servers:
  - host: 172.16.5.250
tici_worker_servers:
  - host: 172.16.5.251
    data_dir: "tici-worker-data"
`), &topo)
	require.NoError(t, err)
	require.Equal(t, "test1", topo.GlobalOptions.User)
	require.Equal(t, 220, topo.GlobalOptions.SSHPort)
	require.Equal(t, 220, topo.TiDBServers[0].SSHPort)
	require.Equal(t, "tidb-deploy", topo.TiDBServers[0].DeployDir)

	require.Equal(t, 220, topo.PDServers[0].SSHPort)
	require.Equal(t, "test-deploy/pd-2379", topo.PDServers[0].DeployDir)
	require.Equal(t, "pd-data", topo.PDServers[0].DataDir)

	require.Equal(t, 220, topo.CDCServers[0].SSHPort)
	require.Equal(t, "test-deploy/cdc-8300", topo.CDCServers[0].DeployDir)
	require.Equal(t, "cdc-data", topo.CDCServers[0].DataDir)

	require.Equal(t, 220, topo.TiKVCDCServers[0].SSHPort)
	require.Equal(t, "test-deploy/tikv-cdc-8600", topo.TiKVCDCServers[0].DeployDir)
	require.Equal(t, "tikv-cdc-data", topo.TiKVCDCServers[0].DataDir)

	require.Equal(t, 220, topo.TiCIMetaServers[0].SSHPort)
	require.Equal(t, "test-deploy/tici-meta-8500", topo.TiCIMetaServers[0].DeployDir)

	require.Equal(t, 220, topo.TiCIWorkerServers[0].SSHPort)
	require.Equal(t, "test-deploy/tici-worker-8510", topo.TiCIWorkerServers[0].DeployDir)
	require.Equal(t, "tici-worker-data", topo.TiCIWorkerServers[0].DataDir)
}

func TestDataDirAbsolute(t *testing.T) {
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
kvcdc_servers:
  - host: 172.16.5.244
    data_dir: "tikv-cdc-data"
  - host: 172.16.5.245
    port: 33333
tici_meta_servers:
  - host: 172.16.5.254
  - host: 172.16.5.255
    port: 8530
tici_worker_servers:
  - host: 172.16.5.264
    data_dir: "/tici-worker-data"
  - host: 172.16.5.265
    port: 8550
`), &topo)
	require.NoError(t, err)

	require.Equal(t, "pd-data", topo.PDServers[0].DataDir)
	require.Equal(t, "/test-data/pd-12379", topo.PDServers[1].DataDir)

	require.Equal(t, "cdc-data", topo.CDCServers[0].DataDir)
	require.Equal(t, "/test-data/cdc-23333", topo.CDCServers[1].DataDir)

	require.Equal(t, "tikv-cdc-data", topo.TiKVCDCServers[0].DataDir)
	require.Equal(t, "/test-data/tikv-cdc-33333", topo.TiKVCDCServers[1].DataDir)

	require.Equal(t, "/tici-worker-data", topo.TiCIWorkerServers[0].DataDir)
	require.Equal(t, "/test-data/tici-worker-8550", topo.TiCIWorkerServers[1].DataDir)
}

func TestGlobalConfig(t *testing.T) {
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
  kvcdc:
    gc-ttl: 43200

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

kvcdc_servers:
  - host: 172.16.5.200
  - host: 172.16.5.201
    port: 8601
    config:
      log-level: "debug"
tici_meta_servers:
  - host: 172.16.5.254
  - host: 172.16.5.255
    config:
      reader_pool.ttl_seconds: 9
tici_worker_servers:
  - host: 172.16.5.256
  - host: 172.16.5.257
    config:
      heartbeat_interval: "10s"
`), &topo)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"status.address":  10,
		"port":            1230,
		"latch.capacity":  20480,
		"log.file.rotate": "123445.xxx",
	}, topo.ServerConfigs.TiDB)
	require.Equal(t, map[string]any{
		"gc-ttl": 43200,
	}, topo.ServerConfigs.TiKVCDC)

	expected := map[string]any{
		"status": map[string]any{
			"address": 10,
		},
		"port": 1230,
		"latch": map[string]any{
			"capacity": 20480,
		},
		"log": map[string]any{
			"file": map[string]any{
				"rotate": "123445.xxx",
			},
		},
	}
	got := FoldMap(topo.ServerConfigs.TiDB)
	require.Equal(t, expected, got)
	buf := &bytes.Buffer{}
	err = toml.NewEncoder(buf).Encode(expected)
	require.NoError(t, err)
	require.Equal(t, `port = 1230

[latch]
  capacity = 20480

[log]
  [log.file]
    rotate = "123445.xxx"

[status]
  address = 10
`, buf.String())

	expected = map[string]any{
		"latch": map[string]any{
			"capacity": 3000,
		},
		"log": map[string]any{
			"file": map[string]any{
				"rotate": "44444.xxx",
			},
		},
	}
	got = FoldMap(topo.TiDBServers[0].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{
		"latch": map[string]any{
			"capacity": 5000,
		},
		"log": map[string]any{
			"file": map[string]any{
				"rotate": "55555.xxx",
			},
		},
	}
	got = FoldMap(topo.TiDBServers[1].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{}
	got = FoldMap(topo.TiKVCDCServers[0].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{}
	got = FoldMap(topo.TiKVCDCServers[0].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{
		"log-level": "debug",
	}
	got = FoldMap(topo.TiKVCDCServers[1].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{}
	got = FoldMap(topo.TiCIMetaServers[0].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{
		"reader_pool": map[string]any{
			"ttl_seconds": 9,
		},
	}
	got = FoldMap(topo.TiCIMetaServers[1].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{}
	got = FoldMap(topo.TiCIWorkerServers[0].Config)
	require.Equal(t, expected, got)

	expected = map[string]any{
		"heartbeat_interval": "10s",
	}
	got = FoldMap(topo.TiCIWorkerServers[1].Config)
	require.Equal(t, expected, got)
}

func TestGlobalConfigPatch(t *testing.T) {
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
	require.NoError(t, err)
	expected := map[string]any{
		"config": map[string]any{
			"item1": 100,
			"item2": 300,
			"item3": map[string]any{
				"item5": 500,
				"item6": 600,
			},
		},
	}
	got := FoldMap(topo.TiKVServers[0].Config)
	require.Equal(t, expected, got)
}

func TestLogDir(t *testing.T) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
tidb_servers:
  - host: 172.16.5.138
    deploy_dir: "test-deploy"
    log_dir: "test-deploy/log"
`), &topo)
	require.NoError(t, err)
	require.Equal(t, "test-deploy/log", topo.TiDBServers[0].LogDir)
}

func TestMonitorLogDir(t *testing.T) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
monitored:
    deploy_dir: "test-deploy"
    log_dir: "test-deploy/log"
`), &topo)
	require.NoError(t, err)
	require.Equal(t, "test-deploy/log", topo.MonitoredOptions.LogDir)

	out, err := yaml.Marshal(topo)
	require.NoError(t, err)
	err = yaml.Unmarshal(out, &topo)
	require.NoError(t, err)
	require.Equal(t, "test-deploy/log", topo.MonitoredOptions.LogDir)
}

func TestMerge2Toml(t *testing.T) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
server_configs:
  tikv:
    config.item1: 100
    config.item2: 300
    config.item3.item5: 500
    config.item3.item6: 600
  kvcdc:
    gc-ttl: 43200

tikv_servers:
  - host: 172.16.5.138
    config:
      config.item2: 500
      config.item3.item5: 700

kvcdc_servers:
  - host: 172.16.5.238
    config:
      log-level: "debug"

`), &topo)
	require.NoError(t, err)
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
	got, err := Merge2Toml("tikv", topo.ServerConfigs.TiKV, topo.TiKVServers[0].Config)
	require.NoError(t, err)
	require.Equal(t, expected, string(got))

	expected = `# WARNING: This file is auto-generated. Do not edit! All your modification will be overwritten!
# You can use 'tiup cluster edit-config' and 'tiup cluster reload' to update the configuration
# All configuration items you want to change can be added to:
# server_configs:
#   kvcdc:
#     aa.b1.c3: value
#     aa.b2.c4: value
gc-ttl = 43200
log-level = "debug"
`
	got, err = Merge2Toml("kvcdc", topo.ServerConfigs.TiKVCDC, topo.TiKVCDCServers[0].Config)
	require.NoError(t, err)
	require.Equal(t, expected, string(got))
}

func TestMerge2Toml2(t *testing.T) {
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
	require.NoError(t, err)
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
	got, err := Merge2Toml("pd", topo.ServerConfigs.PD, topo.PDServers[1].Config)
	require.NoError(t, err)
	require.Equal(t, expected, string(got))
}

func TestTiKVLabels(t *testing.T) {
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
	require.NoError(t, err)
	labels, err := spec.TiKVServers[0].Labels()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"dc":   "dc1",
		"zone": "zone1",
		"host": "host1",
	}, labels)

	spec = Specification{}
	err = yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.138
    config:
      server.labels.dc: dc1
      server.labels.zone: zone1
      server.labels.host: host1
`), &spec)
	require.NoError(t, err)
	/*
		labels, err = spec.TiKVServers[0].Labels()
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"dc":   "dc1",
			"zone": "zone1",
			"host": "host1",
		}, labels)
	*/
}

func TestLocationLabels(t *testing.T) {
	spec := Specification{}

	lbs, err := spec.LocationLabels()
	require.NoError(t, err)
	require.Equal(t, 0, len(lbs))

	err = yaml.Unmarshal([]byte(`
server_configs:
  pd:
    replication.location-labels: ["zone", "host"]
`), &spec)
	require.NoError(t, err)
	lbs, err = spec.LocationLabels()
	require.NoError(t, err)
	require.Equal(t, []string{"zone", "host"}, lbs)

	spec = Specification{}
	err = yaml.Unmarshal([]byte(`
server_configs:
  pd:
    replication:
      location-labels:
        - zone
        - host
`), &spec)
	require.NoError(t, err)
	lbs, err = spec.LocationLabels()
	require.NoError(t, err)
	require.Equal(t, []string{"zone", "host"}, lbs)

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
	require.NoError(t, err)
	_, err = spec.LocationLabels()
	require.Error(t, err)
}

func TestTiFlashRequiredCPUFlags(t *testing.T) {
	obtained := getTiFlashRequiredCPUFlagsWithVersion("v6.3.0", "AMD64")
	require.Equal(t, TiFlashRequiredCPUFlags, obtained)
	obtained = getTiFlashRequiredCPUFlagsWithVersion("v6.3.0", "X86_64")
	require.Equal(t, TiFlashRequiredCPUFlags, obtained)
	obtained = getTiFlashRequiredCPUFlagsWithVersion("nightly", "amd64")
	require.Equal(t, TiFlashRequiredCPUFlags, obtained)
	obtained = getTiFlashRequiredCPUFlagsWithVersion("v6.3.0", "aarch64")
	require.Equal(t, "", obtained)
	obtained = getTiFlashRequiredCPUFlagsWithVersion("v6.2.0", "amd64")
	require.Equal(t, "", obtained)
}

func TestTiFlashStorageSection(t *testing.T) {
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
	require.NoError(t, err)

	flashComp := FindComponent(spec, ComponentTiFlash)
	instances := flashComp.Instances()
	require.Equal(t, 1, len(instances))
	// parse using clusterVersion<"v4.0.9"
	{
		ins := instances[0]
		dataDirs := MultiDirAbs("", spec.TiFlashServers[0].DataDir)
		conf, err := ins.(*TiFlashInstance).initTiFlashConfig(ctx, "v4.0.8", spec.ServerConfigs.TiFlash, meta.DirPaths{Deploy: spec.TiFlashServers[0].DeployDir, Data: dataDirs, Log: spec.TiFlashServers[0].LogDir})
		require.NoError(t, err)

		path, ok := conf["path"]
		require.True(t, ok)
		require.Equal(t, "/ssd0/tiflash,/ssd1/tiflash", path)
	}
	// parse using clusterVersion>="v4.0.9"
	checkWithVersion := func(ver string) {
		ins := instances[0].(*TiFlashInstance)
		dataDirs := MultiDirAbs("", spec.TiFlashServers[0].DataDir)
		conf, err := ins.initTiFlashConfig(ctx, ver, spec.ServerConfigs.TiFlash, meta.DirPaths{Deploy: spec.TiFlashServers[0].DeployDir, Data: dataDirs, Log: spec.TiFlashServers[0].LogDir})
		require.NoError(t, err)

		_, ok := conf["path"]
		require.True(t, ok)

		// After merging instance configurations with "storgae", the "path" property should be removed.
		conf, err = ins.mergeTiFlashInstanceConfig(ver, conf, ins.InstanceSpec.(*TiFlashSpec).Config)
		require.NoError(t, err)
		_, ok = conf["path"]
		require.False(t, ok)

		if storageSection, ok := conf["storage"]; ok {
			if mainSection, ok := storageSection.(map[string]any)["main"]; ok {
				if mainDirsSection, ok := mainSection.(map[string]any)["dir"]; ok {
					var mainDirs []any = mainDirsSection.([]any)
					require.Equal(t, 2, len(mainDirs))
					require.Equal(t, "/ssd0/tiflash", mainDirs[0].(string))
					require.Equal(t, "/ssd1/tiflash", mainDirs[1].(string))
				} else {
					t.Error("Can not get storage.main.dir section")
				}
			} else {
				t.Error("Can not get storage.main section")
			}
			if latestSection, ok := storageSection.(map[string]any)["latest"]; ok {
				if latestDirsSection, ok := latestSection.(map[string]any)["dir"]; ok {
					var latestDirs []any = latestDirsSection.([]any)
					require.Equal(t, 1, len(latestDirs))
					require.Equal(t, "/ssd0/tiflash", latestDirs[0].(string))
				} else {
					t.Error("Can not get storage.main.dir section")
				}
			} else {
				t.Error("Can not get storage.main section")
			}
		} else {
			t.Error("Can not get storage section")
		}
	}
	checkWithVersion("v4.0.9")
	checkWithVersion("nightly")
}

func TestTiFlashInvalidStorageSection(t *testing.T) {
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
		require.Error(t, err)
	}
}

func TestTiCDCDataDir(t *testing.T) {
	spec := &Specification{}
	err := yaml.Unmarshal([]byte(`
cdc_servers:
  - host: 172.16.6.191
    data_dir: /tidb-data/cdc-8300
`), spec)
	require.NoError(t, err)

	cdcComp := FindComponent(spec, ComponentCDC)
	instances := cdcComp.Instances()
	require.Equal(t, 1, len(instances))

	expected := map[string]struct {
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
		"v6.0.0-alpha": {true, true, true},
		"v6.1.0":       {true, true, true},
		"v99.0.0":      {true, true, true},
	}

	checkByVersion := func(version string) {
		ins := instances[0].(*CDCInstance)
		cfg := &scripts.CDCScript{
			DataDirEnabled:    tidbver.TiCDCSupportDataDir(version),
			ConfigFileEnabled: tidbver.TiCDCSupportConfigFile(version),
			TLSEnabled:        false,
			DataDir:           utils.Ternary(tidbver.TiCDCSupportSortOrDataDir(version), ins.DataDir(), "").(string),
		}

		wanted := expected[version]

		require.Equal(t, wanted.configSupported, cfg.ConfigFileEnabled, version)
		require.Equal(t, wanted.dataDirSupported, cfg.DataDirEnabled, version)
		require.Equal(t, wanted.dataDir, len(cfg.DataDir) != 0, version)
	}

	for k := range expected {
		checkByVersion(k)
	}
}

func TestTiFlashUsersSettings(t *testing.T) {
	spec := &Specification{}
	err := yaml.Unmarshal([]byte(`
tiflash_servers:
  - host: 172.16.5.138
    data_dir: /ssd0/tiflash
`), spec)
	require.NoError(t, err)

	ctx := context.Background()

	flashComp := FindComponent(spec, ComponentTiFlash)
	instances := flashComp.Instances()
	require.Equal(t, 1, len(instances))

	// parse using clusterVersion<"v4.0.12" || == "5.0.0-rc"
	checkBackwardCompatibility := func(ver string) {
		ins := instances[0].(*TiFlashInstance)
		dataDirs := MultiDirAbs("", spec.TiFlashServers[0].DataDir)
		conf, err := ins.initTiFlashConfig(ctx, ver, spec.ServerConfigs.TiFlash, meta.DirPaths{Deploy: spec.TiFlashServers[0].DeployDir, Data: dataDirs, Log: spec.TiFlashServers[0].LogDir})
		require.NoError(t, err)

		// We need an empty string for 'users.default.password' for backward compatibility. Or the TiFlash process will fail to start with older versions
		if usersSection, ok := conf["users"]; !ok {
			t.Error("Can not get users section")
		} else {
			if defaultUser, ok := usersSection.(map[string]any)["default"]; !ok {
				t.Error("Can not get default users section")
			} else {
				password := defaultUser.(map[string]any)["password"]
				require.Equal(t, "", password.(string))
			}
		}
	}
	checkBackwardCompatibility("v4.0.11")
	checkBackwardCompatibility("v5.0.0-rc")

	// parse using clusterVersion>="v4.0.12"
	checkWithVersion := func(ver string) {
		ins := instances[0].(*TiFlashInstance)
		dataDirs := MultiDirAbs("", spec.TiFlashServers[0].DataDir)
		conf, err := ins.initTiFlashConfig(ctx, ver, spec.ServerConfigs.TiFlash, meta.DirPaths{Deploy: spec.TiFlashServers[0].DeployDir, Data: dataDirs, Log: spec.TiFlashServers[0].LogDir})
		require.NoError(t, err)

		// Those deprecated settings are ignored in newer versions
		_, ok := conf["users"]
		require.False(t, ok)
	}
	checkWithVersion("v4.0.12")
	checkWithVersion("v5.0.0")
	checkWithVersion("nightly")
}

func TestYAMLAnchor(t *testing.T) {
	topo := Specification{}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(`
global:
  custom:
    tidb_spec: &tidb_spec
      deploy_dir: "test-deploy"
      log_dir: "test-deploy/log"

tidb_servers:
  - <<: *tidb_spec
    host: 172.16.5.138
    deploy_dir: "fake-deploy"
`)))
	decoder.KnownFields(true)
	err := decoder.Decode(&topo)
	require.NoError(t, err)
	require.Equal(t, "172.16.5.138", topo.TiDBServers[0].Host)
	require.Equal(t, "fake-deploy", topo.TiDBServers[0].DeployDir)
	require.Equal(t, "test-deploy/log", topo.TiDBServers[0].LogDir)
}

func TestYAMLAnchorWithUndeclared(t *testing.T) {
	topo := Specification{}
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(`
global:
  custom:
    tidb_spec: &tidb_spec
      deploy_dir: "test-deploy"
      log_dir: "test-deploy/log"
      undeclared: "some stuff"

tidb_servers:
  - <<: *tidb_spec
    host: 172.16.5.138
`)))
	decoder.KnownFields(true)
	err := decoder.Decode(&topo)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "not found"))
}
