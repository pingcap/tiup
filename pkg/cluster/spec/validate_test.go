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

	"github.com/joomcode/errorx"
	. "github.com/pingcap/check"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
	"gopkg.in/yaml.v2"
)

func (s *metaSuiteTopo) TestDirectoryConflicts1(c *C) {
	topo := Specification{}

	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tidb_servers:
  - host: 172.16.5.138
    deploy_dir: "/test-1"
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-1"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/test-1' between 'tidb_servers:172.16.5.138.deploy_dir' and 'pd_servers:172.16.5.138.data_dir'")

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "/test-data"
tikv_servers:
  - host: 172.16.5.138
    data_dir: "test-1"
pd_servers:
  - host: 172.16.5.138
    data_dir: "test-1"
`), &topo)
	c.Assert(err, IsNil)

	// report conflict if a non-import node use same dir as an imported one
	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: deploy
  data_dir: data
tidb_servers:
  - host: 172.16.4.190
    deploy_dir: /home/tidb/deploy
pd_servers:
  - host: 172.16.4.190
    imported: true
    name: pd_ip-172-16-4-190
    deploy_dir: /home/tidb/deploy
    data_dir: /home/tidb/deploy/data.pd
    log_dir: /home/tidb/deploy/log
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/home/tidb/deploy' between 'tidb_servers:172.16.4.190.deploy_dir' and 'pd_servers:172.16.4.190.deploy_dir'")

	// two imported tidb pass the validation, two pd servers (only one is imported) don't
	err = yaml.Unmarshal([]byte(`
global:
  user: "test2"
  ssh_port: 220
  deploy_dir: deploy
  data_dir: /data
tidb_servers:
  - host: 172.16.4.190
    imported: true
    port: 3306
    deploy_dir: /home/tidb/deploy1
  - host: 172.16.4.190
    imported: true
    status_port: 3307
    deploy_dir: /home/tidb/deploy1
pd_servers:
  - host: 172.16.4.190
    imported: true
    name: pd_ip-172-16-4-190
    deploy_dir: /home/tidb/deploy
  - host: 172.16.4.190
    name: pd_ip-172-16-4-190-2
    client_port: 2381
    peer_port: 2382
    deploy_dir: /home/tidb/deploy
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/home/tidb/deploy' between 'pd_servers:172.16.4.190.deploy_dir' and 'pd_servers:172.16.4.190.deploy_dir'")
}

func (s *metaSuiteTopo) TestPortConflicts(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tidb_servers:
  - host: 172.16.5.138
    port: 1234
tikv_servers:
  - host: 172.16.5.138
    status_port: 1234
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "port conflict for '1234' between 'tidb_servers:172.16.5.138.port' and 'tikv_servers:172.16.5.138.status_port'")

	topo = Specification{}
	// tispark_masters has "omitempty" in its tag value
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 1234
tispark_masters:
  - host: 172.16.5.138
    port: 1234
tikv_servers:
  - host: 172.16.5.138
    status_port: 2345
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "port conflict for '1234' between 'tispark_masters:172.16.5.138.port' and 'monitored:172.16.5.138.node_exporter_port'")
}

func (s *metaSuiteTopo) TestPlatformConflicts(c *C) {
	// aarch64 and arm64 are equal
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  os: "linux"
  arch: "aarch64"
tidb_servers:
  - host: 172.16.5.138
    arch: "arm64"
    os: "linux"
tikv_servers:
  - host: 172.16.5.138
    arch: "arm64"
    os: "linux"
`), &topo)
	c.Assert(err, IsNil)

	// different arch defined for the same host
	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  os: "linux"
tidb_servers:
  - host: 172.16.5.138
    arch: "aarch64"
    os: "linux"
tikv_servers:
  - host: 172.16.5.138
    arch: "amd64"
    os: "linux"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "platform mismatch for '172.16.5.138' between 'tidb_servers:linux/arm64' and 'tikv_servers:linux/amd64'")

	// different os defined for the same host
	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  os: "linux"
  arch: "aarch64"
tidb_servers:
  - host: 172.16.5.138
    os: "darwin"
    arch: "arm64"
tikv_servers:
  - host: 172.16.5.138
    arch: "arm64"
    os: "linux"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "platform mismatch for '172.16.5.138' between 'tidb_servers:darwin/arm64' and 'tikv_servers:linux/arm64'")
}

func (s *metaSuiteTopo) TestCountDir(c *C) {
	topo := Specification{}

	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
tikv_servers:
  - host: 172.16.5.138
    data_dir: "/test-data/data-1"
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-data/data-2"
`), &topo)
	c.Assert(err, IsNil)
	cnt := topo.CountDir("172.16.5.138", "/home/test1/test-deploy/pd-2379")
	c.Assert(cnt, Equals, 2)
	cnt = topo.CountDir("172.16.5.138", "") // the default user home
	c.Assert(cnt, Equals, 4)
	cnt = topo.CountDir("172.16.5.138", "/test-data/data")
	c.Assert(cnt, Equals, 0) // should not match partial path

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "/test-deploy"
tikv_servers:
  - host: 172.16.5.138
    data_dir: "/test-data/data-1"
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-data/data-2"
`), &topo)
	c.Assert(err, IsNil)
	cnt = topo.CountDir("172.16.5.138", "/test-deploy/pd-2379")
	c.Assert(cnt, Equals, 2)
	cnt = topo.CountDir("172.16.5.138", "")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.16.5.138", "test-data")
	c.Assert(cnt, Equals, 0)

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "/test-deploy"
  data_dir: "/test-data"
tikv_servers:
  - host: 172.16.5.138
    data_dir: "data-1"
pd_servers:
  - host: 172.16.5.138
    data_dir: "data-2"
  - host: 172.16.5.139
`), &topo)
	c.Assert(err, IsNil)
	// if per-instance data_dir is set, the global data_dir is ignored, and if it
	// is a relative path, it will be under the instance's deploy_dir
	cnt = topo.CountDir("172.16.5.138", "/test-deploy/pd-2379")
	c.Assert(cnt, Equals, 3)
	cnt = topo.CountDir("172.16.5.138", "")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.16.5.139", "/test-data")
	c.Assert(cnt, Equals, 1)

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: deploy
  data_dir: data
tidb_servers:
  - host: 172.16.4.190
    imported: true
    deploy_dir: /home/tidb/deploy
pd_servers:
  - host: 172.16.4.190
    imported: true
    name: pd_ip-172-16-4-190
    deploy_dir: /home/tidb/deploy
    data_dir: /home/tidb/deploy/data.pd
    log_dir: /home/tidb/deploy/log
`), &topo)
	c.Assert(err, IsNil)
	cnt = topo.CountDir("172.16.4.190", "/home/tidb/deploy")
	c.Assert(cnt, Equals, 5)
}

func (s *metaSuiteTopo) TestCountDir2(c *C) {
	file := filepath.Join("testdata", "countdir.yaml")
	meta := ClusterMeta{}
	yamlFile, err := os.ReadFile(file)
	c.Assert(err, IsNil)
	err = yaml.UnmarshalStrict(yamlFile, &meta)
	c.Assert(err, IsNil)
	topo := meta.Topology

	// If the imported dir is somehow containing paths ens with slash,
	// or having multiple slash in it, the count result should not
	// be different.
	cnt := topo.CountDir("172.17.0.4", "/foo/bar/sometidbpath123")
	c.Assert(cnt, Equals, 3)
	cnt = topo.CountDir("172.17.0.4", "/foo/bar/sometidbpath123/")
	c.Assert(cnt, Equals, 3)
	cnt = topo.CountDir("172.17.0.4", "/foo/bar/sometidbpath123//")
	c.Assert(cnt, Equals, 3)
	cnt = topo.CountDir("172.17.0.4", "/foo/bar/sometidbpath123/log")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.17.0.4", "/foo/bar/sometidbpath123//log")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.17.0.4", "/foo/bar/sometidbpath123/log/")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.17.0.4", "/foo/bar/sometidbpath123//log/")
	c.Assert(cnt, Equals, 1)
}

func (s *metaSuiteTopo) TestTiSparkSpecValidation(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    peer_port: 1234
tispark_masters:
  - host: 172.16.5.138
    port: 1235
tispark_workers:
  - host: 172.16.5.138
    port: 1236
  - host: 172.16.5.139
    port: 1235
`), &topo)
	c.Assert(err, IsNil)

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    peer_port: 1234
tispark_masters:
  - host: 172.16.5.138
    port: 1235
  - host: 172.16.5.139
    port: 1235
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "a TiSpark enabled cluster with more than 1 Spark master node is not supported")

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    peer_port: 1234
tispark_workers:
  - host: 172.16.5.138
    port: 1235
  - host: 172.16.5.139
    port: 1235
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "there must be a Spark master node if you want to use the TiSpark component")

	err = yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    peer_port: 1234
tispark_masters:
  - host: 172.16.5.138
    port: 1236
tispark_workers:
  - host: 172.16.5.138
    port: 1235
  - host: 172.16.5.139
    port: 1235
  - host: 172.16.5.139
    port: 1236
    web_port: 8089
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "the host 172.16.5.139 is duplicated: multiple TiSpark workers on the same host is not supported by Spark")
}

func (s *metaSuiteTopo) TestTLSEnabledValidation(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  enable_tls: true
pd_servers:
  - host: 172.16.5.138
    peer_port: 1234
tidb_servers:
  - host: 172.16.5.138
    port: 1235
tikv_servers:
  - host: 172.16.5.138
    port: 1236
`), &topo)
	c.Assert(err, IsNil)

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  enable_tls: true
tidb_servers:
  - host: 172.16.5.138
    port: 1234
tispark_masters:
  - host: 172.16.5.138
    port: 1235
  - host: 172.16.5.139
    port: 1235
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "component tispark is not supported in TLS enabled cluster")
}

func (s *metaSuiteTopo) TestMonitorAgentValidation(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    port: 1234
  - host: 172.16.5.139
    ignore_exporter: true
`), &topo)
	c.Assert(err, IsNil)

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    port: 1234
tikv_servers:
  - host: 172.16.5.138
    ignore_exporter: true
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "ignore_exporter mismatch for '172.16.5.138' between 'tikv_servers:true' and 'pd_servers:false'")
}

func (s *metaSuiteTopo) TestCrossClusterPortConflicts(c *C) {
	topo1 := Specification{}
	err := yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    client_port: 1234
    peer_port: 1235
`), &topo1)
	c.Assert(err, IsNil)

	topo2 := Specification{}
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 9101
  blackbox_exporter_port: 9116
pd_servers:
  - host: 172.16.5.138
    client_port: 2234
    peer_port: 2235
  - host: 172.16.5.139
    client_port: 2236
    peer_port: 2237
    ignore_exporter: true
`), &topo2)
	c.Assert(err, IsNil)

	clsList := make(map[string]Metadata)

	// no port conflict with empty list
	err = CheckClusterPortConflict(clsList, "topo", &topo2)
	c.Assert(err, IsNil)

	clsList["topo1"] = &ClusterMeta{Topology: &topo1}

	// no port conflict
	err = CheckClusterPortConflict(clsList, "topo", &topo2)
	c.Assert(err, IsNil)

	// add topo2 to the list
	clsList["topo2"] = &ClusterMeta{Topology: &topo2}

	// monitoring agent port conflict
	topo3 := Specification{}
	err = yaml.Unmarshal([]byte(`
tidb_servers:
  - host: 172.16.5.138
`), &topo3)
	c.Assert(err, IsNil)
	err = CheckClusterPortConflict(clsList, "topo", &topo3)
	c.Assert(err, NotNil)
	c.Assert(errors.Cause(err).Error(), Equals, "spec.deploy.port_conflict: Deploy port conflicts to an existing cluster")
	suggestion, ok := errorx.ExtractProperty(err, utils.ErrPropSuggestion)
	c.Assert(ok, IsTrue)
	c.Assert(suggestion, Equals, `The port you specified in the topology file is:
  Port:      9100
  Component: monitor 172.16.5.138

It conflicts to a port in the existing cluster:
  Existing Cluster Name: topo1
  Existing Port:         9100
  Existing Component:    monitor 172.16.5.138

Please change to use another port or another host.`)

	// monitoring agent port conflict but the instance marked as ignore_exporter
	topo3 = Specification{}
	err = yaml.Unmarshal([]byte(`
tidb_servers:
- host: 172.16.5.138
  ignore_exporter: true
`), &topo3)
	c.Assert(err, IsNil)
	err = CheckClusterPortConflict(clsList, "topo", &topo3)
	c.Assert(err, IsNil)

	// monitoring agent port conflict but the existing instance marked as ignore_exporter
	topo3 = Specification{}
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 9102
  blackbox_exporter_port: 9116
tidb_servers:
- host: 172.16.5.139
`), &topo3)
	c.Assert(err, IsNil)
	err = CheckClusterPortConflict(clsList, "topo", &topo3)
	c.Assert(err, IsNil)

	// component port conflict
	topo4 := Specification{}
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 9102
  blackbox_exporter_port: 9117
pump_servers:
  - host: 172.16.5.138
    port: 2235
`), &topo4)
	c.Assert(err, IsNil)
	err = CheckClusterPortConflict(clsList, "topo", &topo4)
	c.Assert(err, NotNil)
	c.Assert(errors.Cause(err).Error(), Equals, "spec.deploy.port_conflict: Deploy port conflicts to an existing cluster")
	suggestion, ok = errorx.ExtractProperty(err, utils.ErrPropSuggestion)
	c.Assert(ok, IsTrue)
	c.Assert(suggestion, Equals, `The port you specified in the topology file is:
  Port:      2235
  Component: pump 172.16.5.138

It conflicts to a port in the existing cluster:
  Existing Cluster Name: topo2
  Existing Port:         2235
  Existing Component:    pd 172.16.5.138

Please change to use another port or another host.`)
}

func (s *metaSuiteTopo) TestCrossClusterDirConflicts(c *C) {
	topo1 := Specification{}
	err := yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    client_port: 1234
    peer_port: 1235
`), &topo1)
	c.Assert(err, IsNil)

	topo2 := Specification{}
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 9101
  blackbox_exporter_port: 9116
pd_servers:
  - host: 172.16.5.138
    client_port: 2234
    peer_port: 2235
`), &topo2)
	c.Assert(err, IsNil)

	clsList := make(map[string]Metadata)

	// no port conflict with empty list
	err = CheckClusterDirConflict(clsList, "topo", &topo2)
	c.Assert(err, IsNil)

	clsList["topo1"] = &ClusterMeta{Topology: &topo1}

	// no port conflict
	err = CheckClusterDirConflict(clsList, "topo", &topo2)
	c.Assert(err, IsNil)

	// add topo2 to the list
	clsList["topo2"] = &ClusterMeta{Topology: &topo2}

	// monitoring agent dir conflict
	topo3 := Specification{}
	err = yaml.Unmarshal([]byte(`
tidb_servers:
  - host: 172.16.5.138
`), &topo3)
	c.Assert(err, IsNil)
	err = CheckClusterDirConflict(clsList, "topo", &topo3)
	c.Assert(err, NotNil)
	c.Assert(errors.Cause(err).Error(), Equals, "spec.deploy.dir_conflict: Deploy directory conflicts to an existing cluster")
	suggestion, ok := errorx.ExtractProperty(err, utils.ErrPropSuggestion)
	c.Assert(ok, IsTrue)
	c.Assert(suggestion, Equals, `The directory you specified in the topology file is:
  Directory: monitor deploy directory /home/tidb/deploy/monitor-9100
  Component: tidb 172.16.5.138

It conflicts to a directory in the existing cluster:
  Existing Cluster Name: topo1
  Existing Directory:    monitor deploy directory /home/tidb/deploy/monitor-9100
  Existing Component:    pd 172.16.5.138

Please change to use another directory or another host.`)

	// no dir conflict error if one of the instance is marked as ignore_exporter
	err = yaml.Unmarshal([]byte(`
tidb_servers:
  - host: 172.16.5.138
    ignore_exporter: true
`), &topo3)
	c.Assert(err, IsNil)
	err = CheckClusterDirConflict(clsList, "topo", &topo3)
	c.Assert(err, IsNil)

	// component with different port has no dir conflict
	topo4 := Specification{}
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 9102
  blackbox_exporter_port: 9117
pump_servers:
  - host: 172.16.5.138
    port: 2235
`), &topo4)
	c.Assert(err, IsNil)
	err = CheckClusterDirConflict(clsList, "topo", &topo4)
	c.Assert(err, IsNil)

	// component with reletive dir has no dir conflic
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 9102
  blackbox_exporter_port: 9117
pump_servers:
  - host: 172.16.5.138
    port: 2235
    data_dir: "pd-1234"
`), &topo4)
	c.Assert(err, IsNil)
	err = CheckClusterDirConflict(clsList, "topo", &topo4)
	c.Assert(err, IsNil)

	// component with absolute dir conflict
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 9102
  blackbox_exporter_port: 9117
pump_servers:
  - host: 172.16.5.138
    port: 2235
    data_dir: "/home/tidb/deploy/pd-2234"
`), &topo4)
	c.Assert(err, IsNil)
	err = CheckClusterDirConflict(clsList, "topo", &topo4)
	c.Assert(err, NotNil)
	c.Assert(errors.Cause(err).Error(), Equals, "spec.deploy.dir_conflict: Deploy directory conflicts to an existing cluster")
	suggestion, ok = errorx.ExtractProperty(err, utils.ErrPropSuggestion)
	c.Assert(ok, IsTrue)
	c.Assert(suggestion, Equals, `The directory you specified in the topology file is:
  Directory: data directory /home/tidb/deploy/pd-2234
  Component: pump 172.16.5.138

It conflicts to a directory in the existing cluster:
  Existing Cluster Name: topo2
  Existing Directory:    deploy directory /home/tidb/deploy/pd-2234
  Existing Component:    pd 172.16.5.138

Please change to use another directory or another host.`)
}

func (s *metaSuiteTopo) TestRelativePathDetect(c *C) {
	servers := map[string]string{
		"monitoring_servers":   "rule_dir",
		"grafana_servers":      "dashboard_dir",
		"alertmanager_servers": "config_file",
	}
	paths := map[string]Checker{
		"/an/absolute/path":   IsNil,
		"an/relative/path":    NotNil,
		"./an/relative/path":  NotNil,
		"../an/relative/path": NotNil,
	}

	for server, field := range servers {
		for p, checker := range paths {
			topo5 := Specification{}
			content := fmt.Sprintf(`
%s:
  - host: 1.1.1.1
    %s: %s
`, server, field, p)
			c.Assert(yaml.Unmarshal([]byte(content), &topo5), checker)
		}
	}
}

func (s *metaSuiteTopo) TestTiKVLocationLabelsCheck(c *C) {
	// 2 tikv on different host
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
  - host: 172.16.5.139
    port: 20160
    status_port: 20180
`), &topo)
	c.Assert(err, IsNil)
	err = CheckTiKVLabels(nil, &topo)
	c.Assert(err, IsNil)
	err = CheckTiKVLabels([]string{}, &topo)
	c.Assert(err, IsNil)

	// 2 tikv on the same host without label
	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`), &topo)
	c.Assert(err, IsNil)
	err = CheckTiKVLabels(nil, &topo)
	c.Assert(err, NotNil)

	// 2 tikv on the same host with unacquainted label
	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    config:
      server.labels: { zone: "zone1", host: "172.16.5.140" }
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
    config:
      server.labels: { zone: "zone1", host: "172.16.5.140" }
`), &topo)
	c.Assert(err, IsNil)
	err = CheckTiKVLabels(nil, &topo)
	c.Assert(err, NotNil)

	// 2 tikv on the same host with correct label
	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    config:
      server.labels: { zone: "zone1", host: "172.16.5.140" }
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
    config:
      server.labels: { zone: "zone1", host: "172.16.5.140" }
`), &topo)
	c.Assert(err, IsNil)
	err = CheckTiKVLabels([]string{"zone", "host"}, &topo)
	c.Assert(err, IsNil)

	// 2 tikv on the same host with different config style
	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    config:
      server:
        labels: { zone: "zone1", host: "172.16.5.140" }
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
    config:
      server.labels:
        zone: "zone1"
        host: "172.16.5.140"
`), &topo)
	c.Assert(err, IsNil)
	err = CheckTiKVLabels([]string{"zone", "host"}, &topo)
	c.Assert(err, IsNil)
}

func (s *metaSuiteTopo) TestCountDirMultiPath(c *C) {
	topo := Specification{}

	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
tiflash_servers:
  - host: 172.19.0.104
    data_dir: "/home/tidb/birdstorm/data1, /home/tidb/birdstorm/data3"
`), &topo)
	c.Assert(err, IsNil)
	cnt := topo.CountDir("172.19.0.104", "/home/tidb/birdstorm/data1")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm/data2")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm/data3")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm")
	c.Assert(cnt, Equals, 2)

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
tiflash_servers:
  - host: 172.19.0.104
    data_dir: "birdstorm/data1,/birdstorm/data3"
`), &topo)
	c.Assert(err, IsNil)
	cnt = topo.CountDir("172.19.0.104", "/home/test1/test-deploy/tiflash-9000/birdstorm/data1")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.19.0.104", "/birdstorm/data3")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm/data3")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.19.0.104", "/home/test1/test-deploy/tiflash-9000/birdstorm/data3")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.19.0.104", "/birdstorm")
	c.Assert(cnt, Equals, 1)

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
tiflash_servers:
  - host: 172.19.0.104
    data_dir: /data1 # this is ignored
    config:
      # test with these paths
      storage.main.dir: [ /home/tidb/birdstorm/data1,/home/tidb/birdstorm/data3]
`), &topo)
	c.Assert(err, IsNil)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm/data1")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm/data2")
	c.Assert(cnt, Equals, 0)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm/data3")
	c.Assert(cnt, Equals, 1)
	cnt = topo.CountDir("172.19.0.104", "/home/tidb/birdstorm")
	c.Assert(cnt, Equals, 2)
}

func (s *metaSuiteTopo) TestDirectoryConflictsWithMultiDir(c *C) {
	topo := Specification{}

	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tiflash_servers:
  - host: 172.16.5.138
    data_dir: " /test-1, /test-2"
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-2"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/test-2' between 'tiflash_servers:172.16.5.138.data_dir' and 'pd_servers:172.16.5.138.data_dir'")

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tiflash_servers:
  - host: 172.16.5.138
    data_dir: "/test-1,/test-1"
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-2"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/test-1' between 'tiflash_servers:172.16.5.138.data_dir' and 'tiflash_servers:172.16.5.138.data_dir'")
}

func (s *metaSuiteTopo) TestDirectoryConflictsWithTiFlashMultiDir2(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tiflash_servers:
  - host: 172.16.5.138
    data_dir: "/test-1" # this will be overwrite by storage.main.dir
    config:
      storage.main.dir: [ /test-1, /test-2]
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-2"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/test-2' between 'tiflash_servers:172.16.5.138.data_dir' and 'pd_servers:172.16.5.138.data_dir'")

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tiflash_servers:
  - host: 172.16.5.138
    # this will be overwrite by storage.main.dir
    data_dir: "/test-1"
    config:
      storage.main.dir: [ /test-2, /test-2 ] # conflict inside
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-1"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory conflict for '/test-2' between 'tiflash_servers:172.16.5.138.config.storage.main.dir' and 'tiflash_servers:172.16.5.138.config.storage.main.dir'")

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data"
tiflash_servers:
  - host: 172.16.5.138
    data_dir: "/test-1" # this will be overwrite by storage.main.dir
    config:
      # no conflict between main and latest
      storage.main.dir: [ /test-1, /test-2]
      storage.latest.dir: [ /test-1, /test-2]
pd_servers:
  - host: 172.16.5.138
    data_dir: "/test-3"
`), &topo)
	c.Assert(err, IsNil)
}

func (s *metaSuiteTopo) TestPdServerWithSameName(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
pd_servers:
  - host: 172.16.5.138
    peer_port: 1234
    name: name1
  - host: 172.16.5.139
    perr_port: 1234
    name: name2
`), &topo)
	c.Assert(err, IsNil)

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
  pd_servers:
  - host: 172.16.5.138
    peer_port: 1234
    name: name1
  - host: 172.16.5.139
    perr_port: 1234
    name: name1
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "component pd_servers.name is not supported duplicated, the name name1 is duplicated")
}

func (s *metaSuiteTopo) TestInvalidPort(c *C) {
	topo := Specification{}

	err := yaml.Unmarshal([]byte(`
global:
  ssh_port: 65536
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "`global` of ssh_port=65536 is invalid, port should be in the range [1, 65535]")

	err = yaml.Unmarshal([]byte(`
global:
  ssh_port: 655
tidb_servers:
  - host: 172.16.5.138
    port: -1
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "`tidb_servers` of port=-1 is invalid, port should be in the range [1, 65535]")

	err = yaml.Unmarshal([]byte(`
monitored:
    node_exporter_port: 102400
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "`monitored` of node_exporter_port=102400 is invalid, port should be in the range [1, 65535]")
}

func (s *metaSuiteTopo) TestInvalidUserGroup(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: helloworldtidb-_-_
  group: wor_l-d
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.User, Equals, "helloworldtidb-_-_")
	c.Assert(topo.GlobalOptions.Group, Equals, "wor_l-d")

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  user: ../hello
`), &topo)
	c.Assert(err, NotNil)

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  user: hel.lo
`), &topo)
	c.Assert(err, NotNil)

	topo = Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  group: hello123456789012
`), &topo)
	c.Assert(err, NotNil)
}

func (s *metaSuiteTopo) TestMissingGroup(c *C) {
	topo := Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: tidb
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.User, Equals, "tidb")
	c.Assert(topo.GlobalOptions.Group, Equals, "")
}

func (s *metaSuiteTopo) TestLogDirUnderDataDir(c *C) {
	topo := Specification{}
	clsList := make(map[string]Metadata)

	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: deploy
  data_dir: data
tikv_servers:
  - host: n1
    port: 32160
    status_port: 32180
    log_dir: "/home/tidb6wu/tidb1-data/tikv-32160/log"
    data_dir: "/home/tidb6wu/tidb1-data/tikv-32160"
`), &topo)
	c.Assert(err, IsNil)
	err = CheckClusterDirConflict(clsList, "topo", &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "spec.deploy.dir_overlap: Deploy directory overlaps to another instance")
	suggestion, ok := errorx.ExtractProperty(err, utils.ErrPropSuggestion)
	c.Assert(ok, IsTrue)
	c.Assert(suggestion, Equals, `The directory you specified in the topology file is:
  Directory: data directory /home/tidb6wu/tidb1-data/tikv-32160
  Component: tikv n1

It overlaps to another instance:
  Other Directory: log directory /home/tidb6wu/tidb1-data/tikv-32160/log
  Other Component: tikv n1

Please modify the topology file and try again.`)

	goodTopos := []string{
		`
tikv_servers:
  - host: n1
    log_dir: 'tikv-data/log'
  - host: n2
    data_dir: 'tikv-data'
`,
		`
tikv_servers:
  - host: n1
    port: 32160
    status_port: 32180
    log_dir: "/home/tidb6wu/tidb1-data/tikv-32160-log"
    data_dir: "/home/tidb6wu/tidb1-data/tikv-32160"
`,
		`
monitored:
  node_exporter_port: 9100
  blackbox_exporter_port: 9115
  deploy_dir: /data/deploy/monitor-9100
  data_dir: /data/deploy/monitor-9100
  log_dir: /data/deploy/monitor-9100/log
pd_servers:
  - host: n0
    name: pd0
    imported: true
    deploy_dir: /data/deploy
    data_dir: /data/deploy/data.pd
    log_dir: /data/deploy/log
  - host: n1
    name: pd1
    log_dir: "/data/deploy/pd-2379/log"
    data_dir: "/data/pd-2379"
    deploy_dir: "/data/deploy/pd-2379"
cdc_servers:
  - host: n1
    port: 8300
    deploy_dir: /data/deploy/ticdc-8300
    data_dir: /data1/ticdc-8300
    log_dir: /data/deploy/ticdc-8300/log
`,
	}
	for _, s := range goodTopos {
		err = yaml.Unmarshal([]byte(s), &topo)
		c.Assert(err, IsNil)
		err = CheckClusterDirConflict(clsList, "topo", &topo)
		c.Assert(err, IsNil)
	}

	overlapTopos := []string{
		`
tikv_servers:
  - host: n1
    log_dir: 'tikv-data/log'
    data_dir: 'tikv-data'
`,
		`
tikv_servers:
  - host: n1
    log_dir: '/home/tidb6wu/tidb1-data/tikv-32160/log'
    data_dir: '/home/tidb6wu/tidb1-data/tikv-32160'
`,
		`
tikv_servers:
  - host: n1
    log_dir: '/home/tidb6wu/tidb1-data/tikv-32160/log'
    data_dir: '/home/tidb6wu/tidb1-data/tikv-32160/log/data'
`,
		`
tikv_servers:
  - host: n1
    log_dir: 'tikv-log'
    data_dir: 'tikv-log/data'
`,
		`
tikv_servers:
  - host: n1
    data_dir: '/home/tidb6wu/tidb1-data/tikv-32160/log'

tidb_servers:
  - host: n1
    log_dir: '/home/tidb6wu/tidb1-data/tikv-32160/log/data'
`,
		`
tikv_servers:
  - host: n1
    log_dir: '/home/tidb6wu/tidb1-data/tikv-32160/log'

tidb_servers:
  - host: n1
    log_dir: '/home/tidb6wu/tidb1-data/tikv-32160/log/log'
`,
		`
tikv_servers:
  - host: n1
    data_dir: '/home/tidb6wu/tidb1-data/tikv-32160/data'

pd_servers:
  - host: n1
    data_dir: '/home/tidb6wu/tidb1-data/tikv-32160'
`,
		`
global:
  user: "test1"
  deploy_dir: deploy
  data_dir: data
tikv_servers:
  - host: n1
    log_dir: "/home/test1/deploy/tikv-20160/ddd/log"
    data_dir: "ddd"
`,
		`
global:
  user: "test1"
  deploy_dir: deploy
  data_dir: data
tikv_servers:
  - host: n1
    log_dir: "log"
    data_dir: "/home/test1/deploy/tikv-20160/log/data"
`,
	}

	for _, s := range overlapTopos {
		err = yaml.Unmarshal([]byte(s), &topo)
		c.Assert(err, IsNil)
		err = CheckClusterDirConflict(clsList, "topo", &topo)
		c.Assert(err, NotNil)
		c.Assert(err.Error(), Equals, "spec.deploy.dir_overlap: Deploy directory overlaps to another instance")
	}
}
