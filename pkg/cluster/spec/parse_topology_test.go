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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func withTempFile(t *testing.T, content string, fn func(string)) {
	file, err := os.CreateTemp("/tmp", "topology-test")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	_, err = file.WriteString(content)
	require.NoError(t, err)
	file.Close()

	fn(file.Name())
}

func with2TempFile(t *testing.T, content1, content2 string, fn func(string, string)) {
	withTempFile(t, content1, func(file1 string) {
		withTempFile(t, content2, func(file2 string) {
			fn(file1, file2)
		})
	})
}

func TestParseTopologyYaml(t *testing.T) {
	file := filepath.Join("testdata", "topology_err.yaml")
	topo := Specification{}
	err := ParseTopologyYaml(file, &topo)
	require.NoError(t, err)
}

func TestParseTopologyYamlIgnoreGlobal(t *testing.T) {
	file := filepath.Join("testdata", "topology_err.yaml")
	topo := Specification{}
	err := ParseTopologyYaml(file, &topo, true)
	if topo.GlobalOptions.DeployDir == "/tidb/deploy" {
		t.Error("Can not ignore global variables")
	}
	require.NoError(t, err)
}

func TestRelativePath(t *testing.T) {
	// test relative path
	withTempFile(t, `
tikv_servers:
  - host: 172.16.5.140
    deploy_dir: my-deploy
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)
		require.Equal(t, "/home/tidb/my-deploy", topo.TiKVServers[0].DeployDir)
	})

	// test data dir & log dir
	withTempFile(t, `
tikv_servers:
  - host: 172.16.5.140
    deploy_dir: my-deploy
    data_dir: my-data
    log_dir: my-log
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)
		require.Equal(t, "/home/tidb/my-deploy", topo.TiKVServers[0].DeployDir)
		require.Equal(t, "/home/tidb/my-deploy/my-data", topo.TiKVServers[0].DataDir)
		require.Equal(t, "/home/tidb/my-deploy/my-log", topo.TiKVServers[0].LogDir)
	})

	// test global options, case 1
	withTempFile(t, `
global:
  deploy_dir: my-deploy

tikv_servers:
  - host: 172.16.5.140
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)
		require.Equal(t, "my-deploy", topo.GlobalOptions.DeployDir)
		require.Equal(t, "data", topo.GlobalOptions.DataDir)

		require.Equal(t, "/home/tidb/my-deploy/tikv-20160", topo.TiKVServers[0].DeployDir)
		require.Equal(t, "/home/tidb/my-deploy/tikv-20160/data", topo.TiKVServers[0].DataDir)
	})

	// test global options, case 2
	withTempFile(t, `
global:
  deploy_dir: my-deploy

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)
		require.Equal(t, "my-deploy", topo.GlobalOptions.DeployDir)
		require.Equal(t, "data", topo.GlobalOptions.DataDir)

		require.Equal(t, "/home/tidb/my-deploy/tikv-20160", topo.TiKVServers[0].DeployDir)
		require.Equal(t, "/home/tidb/my-deploy/tikv-20160/data", topo.TiKVServers[0].DataDir)

		require.Equal(t, "/home/tidb/my-deploy/tikv-20161", topo.TiKVServers[1].DeployDir)
		require.Equal(t, "/home/tidb/my-deploy/tikv-20161/data", topo.TiKVServers[1].DataDir)
	})

	// test global options, case 3
	withTempFile(t, `
global:
  deploy_dir: my-deploy

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    data_dir: my-data
    log_dir: my-log
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)

		require.Equal(t, "my-deploy/monitor-9100", topo.MonitoredOptions.DeployDir)
		require.Equal(t, "data/monitor-9100", topo.MonitoredOptions.DataDir)
		require.Equal(t, "my-deploy/monitor-9100/log", topo.MonitoredOptions.LogDir)

		ExpandRelativeDir(&topo)
		require.Equal(t, "my-deploy", topo.GlobalOptions.DeployDir)
		require.Equal(t, "data", topo.GlobalOptions.DataDir)

		require.Equal(t, "/home/tidb/my-deploy/monitor-9100", topo.MonitoredOptions.DeployDir)
		require.Equal(t, "/home/tidb/my-deploy/monitor-9100/data/monitor-9100", topo.MonitoredOptions.DataDir)
		require.Equal(t, "/home/tidb/my-deploy/monitor-9100/my-deploy/monitor-9100/log", topo.MonitoredOptions.LogDir)

		require.Equal(t, "/home/tidb/my-deploy/tikv-20160", topo.TiKVServers[0].DeployDir)
		require.Equal(t, "/home/tidb/my-deploy/tikv-20160/my-data", topo.TiKVServers[0].DataDir)
		require.Equal(t, "/home/tidb/my-deploy/tikv-20160/my-log", topo.TiKVServers[0].LogDir)

		require.Equal(t, "/home/tidb/my-deploy/tikv-20161", topo.TiKVServers[1].DeployDir)
		require.Equal(t, "/home/tidb/my-deploy/tikv-20161/data", topo.TiKVServers[1].DataDir)
		require.Equal(t, "/home/tidb/my-deploy/tikv-20161/log", topo.TiKVServers[1].LogDir)
	})

	// test global options, case 4
	withTempFile(t, `
global:
  data_dir: my-global-data
  log_dir: my-global-log

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    data_dir: my-local-data
    log_dir: my-local-log
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)
		require.Equal(t, "deploy", topo.GlobalOptions.DeployDir)
		require.Equal(t, "my-global-data", topo.GlobalOptions.DataDir)
		require.Equal(t, "my-global-log", topo.GlobalOptions.LogDir)

		require.Equal(t, "/home/tidb/deploy/monitor-9100", topo.MonitoredOptions.DeployDir)
		require.Equal(t, "/home/tidb/deploy/monitor-9100/my-global-data/monitor-9100", topo.MonitoredOptions.DataDir)
		require.Equal(t, "/home/tidb/deploy/monitor-9100/deploy/monitor-9100/log", topo.MonitoredOptions.LogDir)

		require.Equal(t, "/home/tidb/deploy/tikv-20160", topo.TiKVServers[0].DeployDir)
		require.Equal(t, "/home/tidb/deploy/tikv-20160/my-local-data", topo.TiKVServers[0].DataDir)
		require.Equal(t, "/home/tidb/deploy/tikv-20160/my-local-log", topo.TiKVServers[0].LogDir)

		require.Equal(t, "/home/tidb/deploy/tikv-20161", topo.TiKVServers[1].DeployDir)
		require.Equal(t, "/home/tidb/deploy/tikv-20161/my-global-data", topo.TiKVServers[1].DataDir)
		require.Equal(t, "/home/tidb/deploy/tikv-20161/my-global-log", topo.TiKVServers[1].LogDir)
	})

	// test multiple dir, case 5
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /path/to/my-first-data,my-second-data
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/path/to/my-first-data,/home/tidb/deploy/tiflash-9000/my-second-data", topo.TiFlashServers[0].DataDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/log", topo.TiFlashServers[0].LogDir)
	})

	// test global options, case 6
	withTempFile(t, `
global:
  user: test
  data_dir: my-global-data
  log_dir: my-global-log

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    deploy_dir: my-local-deploy
    data_dir: my-local-data
    log_dir: my-local-log
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)
		require.Equal(t, "deploy", topo.GlobalOptions.DeployDir)
		require.Equal(t, "my-global-data", topo.GlobalOptions.DataDir)
		require.Equal(t, "my-global-log", topo.GlobalOptions.LogDir)

		require.Equal(t, "/home/test/my-local-deploy", topo.TiKVServers[0].DeployDir)
		require.Equal(t, "/home/test/my-local-deploy/my-local-data", topo.TiKVServers[0].DataDir)
		require.Equal(t, "/home/test/my-local-deploy/my-local-log", topo.TiKVServers[0].LogDir)

		require.Equal(t, "/home/test/deploy/tikv-20161", topo.TiKVServers[1].DeployDir)
		require.Equal(t, "/home/test/deploy/tikv-20161/my-global-data", topo.TiKVServers[1].DataDir)
		require.Equal(t, "/home/test/deploy/tikv-20161/my-global-log", topo.TiKVServers[1].LogDir)
	})
}

func TestTiFlashStorage(t *testing.T) {
	// test tiflash storage section, 'storage.main.dir' should not be defined in server_configs
	withTempFile(t, `
server_configs:
  tiflash:
    storage.main.dir: [/data1/tiflash]
tiflash_servers:
  - host: 172.16.5.140
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.Error(t, err)
	})

	// test tiflash storage section, 'storage.latest.dir' should not be defined in server_configs
	withTempFile(t, `
server_configs:
  tiflash:
    storage.latest.dir: [/data1/tiflash]
tiflash_servers:
  - host: 172.16.5.140
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.Error(t, err)
	})

	// test tiflash storage section defined data dir
	// test for deprecated setting, for backward compatibility
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/ssd0/tiflash", topo.TiFlashServers[0].DataDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/log", topo.TiFlashServers[0].LogDir)
	})

	// test tiflash storage section defined data dir
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash,/ssd1/tiflash,/ssd2/tiflash
    config:
      storage.main.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/ssd0/tiflash,/ssd1/tiflash,/ssd2/tiflash", topo.TiFlashServers[0].DataDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/log", topo.TiFlashServers[0].LogDir)
	})

	// test tiflash storage section defined data dir, "data_dir" will be ignored
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    # if storage.main.dir is defined, data_dir will be ignored
    data_dir: /hdd0/tiflash
    config:
      storage.main.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/ssd0/tiflash,/ssd1/tiflash,/ssd2/tiflash", topo.TiFlashServers[0].DataDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/log", topo.TiFlashServers[0].LogDir)
	})

	// test tiflash storage section defined data dir
	// if storage.latest.dir is not empty, the first path in
	// storage.latest.dir will be the first path in 'DataDir'
	// DataDir is the union set of storage.latest.dir and storage.main.dir
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      storage.main.dir: [/hdd0/tiflash, /hdd1/tiflash, /hdd2/tiflash]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/ssd0/tiflash,/hdd0/tiflash,/hdd1/tiflash,/hdd2/tiflash,/ssd1/tiflash,/ssd2/tiflash", topo.TiFlashServers[0].DataDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/log", topo.TiFlashServers[0].LogDir)
	})

	// test if there is only one path in storage.main.dir
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /hhd0/tiflash
    config:
      storage.main.dir: [/ssd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
		ExpandRelativeDir(&topo)

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/ssd0/tiflash", topo.TiFlashServers[0].DataDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/log", topo.TiFlashServers[0].LogDir)
	})

	// test tiflash storage.latest section defined data dir
	// should always define storage.main.dir if 'storage.latest' is defined
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      #storage.main.dir: [/hdd0/tiflash, /hdd1/tiflash, /hdd2/tiflash]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.Error(t, err)
	})

	// test tiflash storage.raft section defined data dir
	// should always define storage.main.dir if 'storage.raft' is defined
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      #storage.main.dir: [/hdd0/tiflash, /hdd1/tiflash, /hdd2/tiflash]
      storage.raft.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.Error(t, err)
	})

	// test tiflash storage.remote section defined data dir
	// should be fine even when `storage.main.dir` is not defined.
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      storage.remote.dir: /tmp/tiflash/remote
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.NoError(t, err)
	})

	// test tiflash storage section defined data dir
	// storage.main.dir should always use absolute path
	withTempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      storage.main.dir: [tiflash/data, ]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		require.Error(t, err)
	})
}

func merge4test(base, scale string) (*Specification, error) {
	baseTopo := Specification{}
	if err := ParseTopologyYaml(base, &baseTopo); err != nil {
		return nil, err
	}

	scaleTopo := baseTopo.NewPart()
	if err := ParseTopologyYaml(scale, scaleTopo); err != nil {
		return nil, err
	}

	mergedTopo := baseTopo.MergeTopo(scaleTopo)
	if err := mergedTopo.Validate(); err != nil {
		return nil, err
	}

	return mergedTopo.(*Specification), nil
}

func TestTopologyMerge(t *testing.T) {
	// base test
	with2TempFile(t, `
tiflash_servers:
  - host: 172.16.5.140
`, `
tiflash_servers:
  - host: 172.16.5.139
`, func(base, scale string) {
		topo, err := merge4test(base, scale)
		require.NoError(t, err)
		ExpandRelativeDir(topo)
		ExpandRelativeDir(topo) // should be idempotent

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/data", topo.TiFlashServers[0].DataDir)

		require.Equal(t, "/home/tidb/deploy/tiflash-9000", topo.TiFlashServers[1].DeployDir)
		require.Equal(t, "/home/tidb/deploy/tiflash-9000/data", topo.TiFlashServers[1].DataDir)
	})

	// test global option overwrite
	with2TempFile(t, `
global:
  user: test
  deploy_dir: /my-global-deploy

tiflash_servers:
  - host: 172.16.5.140
    log_dir: my-local-log-tiflash
    data_dir: my-local-data-tiflash
  - host: 172.16.5.175
    deploy_dir: flash-deploy
  - host: 172.16.5.141
`, `
tiflash_servers:
  - host: 172.16.5.139
    deploy_dir: flash-deploy
  - host: 172.16.5.134
`, func(base, scale string) {
		topo, err := merge4test(base, scale)
		require.NoError(t, err)

		ExpandRelativeDir(topo)

		require.Equal(t, "/my-global-deploy/tiflash-9000", topo.TiFlashServers[0].DeployDir)
		require.Equal(t, "/my-global-deploy/tiflash-9000/my-local-data-tiflash", topo.TiFlashServers[0].DataDir)
		require.Equal(t, "/my-global-deploy/tiflash-9000/my-local-log-tiflash", topo.TiFlashServers[0].LogDir)

		require.Equal(t, "/home/test/flash-deploy", topo.TiFlashServers[1].DeployDir)
		require.Equal(t, "/home/test/flash-deploy/data", topo.TiFlashServers[1].DataDir)
		require.Equal(t, "/home/test/flash-deploy", topo.TiFlashServers[3].DeployDir)
		require.Equal(t, "/home/test/flash-deploy/data", topo.TiFlashServers[3].DataDir)

		require.Equal(t, "/my-global-deploy/tiflash-9000", topo.TiFlashServers[2].DeployDir)
		require.Equal(t, "/my-global-deploy/tiflash-9000/data", topo.TiFlashServers[2].DataDir)
		require.Equal(t, "/my-global-deploy/tiflash-9000", topo.TiFlashServers[4].DeployDir)
		require.Equal(t, "/my-global-deploy/tiflash-9000/data", topo.TiFlashServers[4].DataDir)
	})
}

func TestMergeComponentVersions(t *testing.T) {
	// test component version overwrite
	with2TempFile(t, `
component_versions:
  tidb: v8.0.0
  tikv: v8.0.0
tidb_servers:
  - host: 172.16.5.139
`, `
component_versions:
  tikv: v8.1.0
  pd: v8.0.0
tidb_servers:
  - host: 172.16.5.134
`, func(base, scale string) {
		baseTopo := Specification{}
		require.NoError(t, ParseTopologyYaml(base, &baseTopo))

		scaleTopo := baseTopo.NewPart()
		require.NoError(t, ParseTopologyYaml(scale, scaleTopo))

		mergedTopo := baseTopo.MergeTopo(scaleTopo)
		require.NoError(t, mergedTopo.Validate())

		require.Equal(t, scaleTopo.(*Specification).ComponentVersions, mergedTopo.(*Specification).ComponentVersions)
		require.Equal(t, "v8.0.0", scaleTopo.(*Specification).ComponentVersions.TiDB)
		require.Equal(t, "v8.1.0", scaleTopo.(*Specification).ComponentVersions.TiKV)
		require.Equal(t, "v8.0.0", scaleTopo.(*Specification).ComponentVersions.PD)
	})
}

func TestFixRelativePath(t *testing.T) {
	// base test
	topo := Specification{
		TiKVServers: []*TiKVSpec{
			{
				DeployDir: "my-deploy",
			},
		},
	}
	expandRelativePath("tidb", &topo)
	require.Equal(t, "/home/tidb/my-deploy", topo.TiKVServers[0].DeployDir)

	// test data dir & log dir
	topo = Specification{
		TiKVServers: []*TiKVSpec{
			{
				DeployDir: "my-deploy",
				DataDir:   "my-data",
				LogDir:    "my-log",
			},
		},
	}
	expandRelativePath("tidb", &topo)
	require.Equal(t, "/home/tidb/my-deploy", topo.TiKVServers[0].DeployDir)
	require.Equal(t, "/home/tidb/my-deploy/my-data", topo.TiKVServers[0].DataDir)
	require.Equal(t, "/home/tidb/my-deploy/my-log", topo.TiKVServers[0].LogDir)

	// test global options
	topo = Specification{
		GlobalOptions: GlobalOptions{
			DeployDir: "my-deploy",
			DataDir:   "my-data",
			LogDir:    "my-log",
		},
		TiKVServers: []*TiKVSpec{
			{},
		},
	}
	expandRelativePath("tidb", &topo)
	require.Equal(t, "my-deploy", topo.GlobalOptions.DeployDir)
	require.Equal(t, "my-data", topo.GlobalOptions.DataDir)
	require.Equal(t, "my-log", topo.GlobalOptions.LogDir)
	require.Equal(t, "", topo.TiKVServers[0].DeployDir)
	require.Equal(t, "", topo.TiKVServers[0].DataDir)
	require.Equal(t, "", topo.TiKVServers[0].LogDir)
}
