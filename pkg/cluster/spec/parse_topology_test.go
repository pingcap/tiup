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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/check"
)

func TestUtils(t *testing.T) {
	check.TestingT(t)
}

type topoSuite struct{}

var _ = check.Suite(&topoSuite{})

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

func with2TempFile(content1, content2 string, fn func(string, string)) {
	withTempFile(content1, func(file1 string) {
		withTempFile(content2, func(file2 string) {
			fn(file1, file2)
		})
	})
}

func (s *topoSuite) TestParseTopologyYaml(c *check.C) {
	file := filepath.Join("testdata", "topology_err.yaml")
	topo := Specification{}
	err := ParseTopologyYaml(file, &topo)
	c.Assert(err, check.IsNil)
}

func (s *topoSuite) TestParseTopologyYamlIgnoreGlobal(c *check.C) {
	file := filepath.Join("testdata", "topology_err.yaml")
	topo := Specification{}
	err := ParseTopologyYaml(file, &topo, true)
	if topo.GlobalOptions.DeployDir == "/tidb/deploy" {
		c.Error("Can not ignore global variables")
	}
	c.Assert(err, check.IsNil)
}

func (s *topoSuite) TestRelativePath(c *check.C) {
	// test relative path
	withTempFile(`
tikv_servers:
  - host: 172.16.5.140
    deploy_dir: my-deploy
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)
		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")
	})

	// test data dir & log dir
	withTempFile(`
tikv_servers:
  - host: 172.16.5.140
    deploy_dir: my-deploy
    data_dir: my-data
    log_dir: my-log
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)
		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/my-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/my-deploy/my-log")
	})

	// test global options, case 1
	withTempFile(`
global:
  deploy_dir: my-deploy

tikv_servers:
  - host: 172.16.5.140
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "my-deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "data")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/data")
	})

	// test global options, case 2
	withTempFile(`
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
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "my-deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "data")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/data")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20161/data")
	})

	// test global options, case 3
	withTempFile(`
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
		c.Assert(err, check.IsNil)

		c.Assert(topo.MonitoredOptions.DeployDir, check.Equals, "my-deploy/monitor-9100")
		c.Assert(topo.MonitoredOptions.DataDir, check.Equals, "data/monitor-9100")
		c.Assert(topo.MonitoredOptions.LogDir, check.Equals, "my-deploy/monitor-9100/log")

		ExpandRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "my-deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "data")

		c.Assert(topo.MonitoredOptions.DeployDir, check.Equals, "/home/tidb/my-deploy/monitor-9100")
		c.Assert(topo.MonitoredOptions.DataDir, check.Equals, "/home/tidb/my-deploy/monitor-9100/data/monitor-9100")
		c.Assert(topo.MonitoredOptions.LogDir, check.Equals, "/home/tidb/my-deploy/monitor-9100/my-deploy/monitor-9100/log")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/my-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/my-log")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20161/data")
		c.Assert(topo.TiKVServers[1].LogDir, check.Equals, "/home/tidb/my-deploy/tikv-20161/log")
	})

	// test global options, case 4
	withTempFile(`
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
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "my-global-data")
		c.Assert(topo.GlobalOptions.LogDir, check.Equals, "my-global-log")

		c.Assert(topo.MonitoredOptions.DeployDir, check.Equals, "/home/tidb/deploy/monitor-9100")
		c.Assert(topo.MonitoredOptions.DataDir, check.Equals, "/home/tidb/deploy/monitor-9100/my-global-data/monitor-9100")
		c.Assert(topo.MonitoredOptions.LogDir, check.Equals, "/home/tidb/deploy/monitor-9100/deploy/monitor-9100/log")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/deploy/tikv-20160/my-local-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/deploy/tikv-20160/my-local-log")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/tidb/deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/tidb/deploy/tikv-20161/my-global-data")
		c.Assert(topo.TiKVServers[1].LogDir, check.Equals, "/home/tidb/deploy/tikv-20161/my-global-log")
	})

	// test multiple dir, case 5
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /path/to/my-first-data,my-second-data
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/path/to/my-first-data,/home/tidb/deploy/tiflash-9000/my-second-data")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "/home/tidb/deploy/tiflash-9000/log")
	})

	// test global options, case 6
	withTempFile(`
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
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "my-global-data")
		c.Assert(topo.GlobalOptions.LogDir, check.Equals, "my-global-log")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/test/my-local-deploy")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/test/my-local-deploy/my-local-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/test/my-local-deploy/my-local-log")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/test/deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/test/deploy/tikv-20161/my-global-data")
		c.Assert(topo.TiKVServers[1].LogDir, check.Equals, "/home/test/deploy/tikv-20161/my-global-log")
	})
}

func (s *topoSuite) TestTiFlashStorage(c *check.C) {
	// test tiflash storage section, 'storage.main.dir' should not be defined in server_configs
	withTempFile(`
server_configs:
  tiflash:
    storage.main.dir: [/data1/tiflash]
tiflash_servers:
  - host: 172.16.5.140
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.NotNil)
	})

	// test tiflash storage section, 'storage.latest.dir' should not be defined in server_configs
	withTempFile(`
server_configs:
  tiflash:
    storage.latest.dir: [/data1/tiflash]
tiflash_servers:
  - host: 172.16.5.140
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.NotNil)
	})

	// test tiflash storage section defined data dir
	// test for depreacated setting, for backward compatibility
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/ssd0/tiflash")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "/home/tidb/deploy/tiflash-9000/log")
	})

	// test tiflash storage section defined data dir
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash,/ssd1/tiflash,/ssd2/tiflash
    config:
      storage.main.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/ssd0/tiflash,/ssd1/tiflash,/ssd2/tiflash")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "/home/tidb/deploy/tiflash-9000/log")
	})

	// test tiflash storage section defined data dir, "data_dir" will be ignored
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    # if storage.main.dir is defined, data_dir will be ignored
    data_dir: /hdd0/tiflash
    config:
      storage.main.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/ssd0/tiflash,/ssd1/tiflash,/ssd2/tiflash")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "/home/tidb/deploy/tiflash-9000/log")
	})

	// test tiflash storage section defined data dir
	// if storage.latest.dir is not empty, the first path in
	// storage.latest.dir will be the first path in 'DataDir'
	// DataDir is the union set of storage.latest.dir and storage.main.dir
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      storage.main.dir: [/hdd0/tiflash, /hdd1/tiflash, /hdd2/tiflash]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/ssd0/tiflash,/hdd0/tiflash,/hdd1/tiflash,/hdd2/tiflash,/ssd1/tiflash,/ssd2/tiflash")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "/home/tidb/deploy/tiflash-9000/log")
	})

	// test if there is only one path in storage.main.dir
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /hhd0/tiflash
    config:
      storage.main.dir: [/ssd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(&topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/ssd0/tiflash")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "/home/tidb/deploy/tiflash-9000/log")
	})

	// test tiflash storage.latest section defined data dir
	// should always define storage.main.dir if 'storage.latest' is defined
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      #storage.main.dir: [/hdd0/tiflash, /hdd1/tiflash, /hdd2/tiflash]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.NotNil)
	})

	// test tiflash storage.raft section defined data dir
	// should always define storage.main.dir if 'storage.raft' is defined
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      #storage.main.dir: [/hdd0/tiflash, /hdd1/tiflash, /hdd2/tiflash]
      storage.raft.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.NotNil)
	})

	// test tiflash storage.remote section defined data dir
	// should be fine even when `storage.main.dir` is not defined.
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      storage.remote.dir: /tmp/tiflash/remote
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
	})

	// test tiflash storage section defined data dir
	// storage.main.dir should always use absolute path
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /ssd0/tiflash
    config:
      storage.main.dir: [tiflash/data, ]
      storage.latest.dir: [/ssd0/tiflash, /ssd1/tiflash, /ssd2/tiflash, /hdd0/tiflash]
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.NotNil)
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

func (s *topoSuite) TestTopologyMerge(c *check.C) {
	// base test
	with2TempFile(`
tiflash_servers:
  - host: 172.16.5.140
`, `
tiflash_servers:
  - host: 172.16.5.139
`, func(base, scale string) {
		topo, err := merge4test(base, scale)
		c.Assert(err, check.IsNil)
		ExpandRelativeDir(topo)
		ExpandRelativeDir(topo) // should be idempotent

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/home/tidb/deploy/tiflash-9000/data")

		c.Assert(topo.TiFlashServers[1].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[1].DataDir, check.Equals, "/home/tidb/deploy/tiflash-9000/data")
	})

	// test global option overwrite
	with2TempFile(`
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
		c.Assert(err, check.IsNil)

		ExpandRelativeDir(topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/my-global-deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/my-global-deploy/tiflash-9000/my-local-data-tiflash")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "/my-global-deploy/tiflash-9000/my-local-log-tiflash")

		c.Assert(topo.TiFlashServers[1].DeployDir, check.Equals, "/home/test/flash-deploy")
		c.Assert(topo.TiFlashServers[1].DataDir, check.Equals, "/home/test/flash-deploy/data")
		c.Assert(topo.TiFlashServers[3].DeployDir, check.Equals, "/home/test/flash-deploy")
		c.Assert(topo.TiFlashServers[3].DataDir, check.Equals, "/home/test/flash-deploy/data")

		c.Assert(topo.TiFlashServers[2].DeployDir, check.Equals, "/my-global-deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[2].DataDir, check.Equals, "/my-global-deploy/tiflash-9000/data")
		c.Assert(topo.TiFlashServers[4].DeployDir, check.Equals, "/my-global-deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[4].DataDir, check.Equals, "/my-global-deploy/tiflash-9000/data")
	})
}

func (s *topoSuite) TestMergeComponentVersions(c *check.C) {
	// test component version overwrite
	with2TempFile(`
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
		c.Assert(ParseTopologyYaml(base, &baseTopo), check.IsNil)

		scaleTopo := baseTopo.NewPart()
		c.Assert(ParseTopologyYaml(scale, scaleTopo), check.IsNil)

		mergedTopo := baseTopo.MergeTopo(scaleTopo)
		c.Assert(mergedTopo.Validate(), check.IsNil)

		c.Assert(scaleTopo.(*Specification).ComponentVersions, check.Equals, mergedTopo.(*Specification).ComponentVersions)
		c.Assert(scaleTopo.(*Specification).ComponentVersions.TiDB, check.Equals, "v8.0.0")
		c.Assert(scaleTopo.(*Specification).ComponentVersions.TiKV, check.Equals, "v8.1.0")
		c.Assert(scaleTopo.(*Specification).ComponentVersions.PD, check.Equals, "v8.0.0")
	})
}

func (s *topoSuite) TestFixRelativePath(c *check.C) {
	// base test
	topo := Specification{
		TiKVServers: []*TiKVSpec{
			{
				DeployDir: "my-deploy",
			},
		},
	}
	expandRelativePath("tidb", &topo)
	c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")

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
	c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")
	c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/my-data")
	c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/my-deploy/my-log")

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
	c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "my-deploy")
	c.Assert(topo.GlobalOptions.DataDir, check.Equals, "my-data")
	c.Assert(topo.GlobalOptions.LogDir, check.Equals, "my-log")
	c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "")
	c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "")
	c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "")
}
