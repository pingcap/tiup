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

	"github.com/joomcode/errorx"
	. "github.com/pingcap/check"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/errutil"
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
tikv_servers:
  - host: 172.16.5.138
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
tikv_servers:
  - host: 172.16.5.138
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
tikv_servers:
  - host: 172.16.5.138
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
	suggestion, ok := errorx.ExtractProperty(err, errutil.ErrPropSuggestion)
	c.Assert(ok, IsTrue)
	c.Assert(suggestion, Equals, `The port you specified in the topology file is:
  Port:      9100
  Component: tidb 172.16.5.138

It conflicts to a port in the existing cluster:
  Existing Cluster Name: topo1
  Existing Port:         9100
  Existing Component:    pd 172.16.5.138

Please change to use another port or another host.`)

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
	suggestion, ok = errorx.ExtractProperty(err, errutil.ErrPropSuggestion)
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
	suggestion, ok := errorx.ExtractProperty(err, errutil.ErrPropSuggestion)
	c.Assert(ok, IsTrue)
	c.Assert(suggestion, Equals, `The directory you specified in the topology file is:
  Directory: monitor deploy directory /home/tidb/deploy/monitor-9100
  Component: tidb 172.16.5.138

It conflicts to a directory in the existing cluster:
  Existing Cluster Name: topo1
  Existing Directory:    monitor deploy directory /home/tidb/deploy/monitor-9100
  Existing Component:    pd 172.16.5.138

Please change to use another directory or another host.`)

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
	suggestion, ok = errorx.ExtractProperty(err, errutil.ErrPropSuggestion)
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
	err = CheckTiKVLocationLabels(nil, &topo)
	c.Assert(err, IsNil)
	err = CheckTiKVLocationLabels([]string{}, &topo)
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
	err = CheckTiKVLocationLabels(nil, &topo)
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
	err = CheckTiKVLocationLabels(nil, &topo)
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
	err = CheckTiKVLocationLabels([]string{"zone", "host"}, &topo)
	c.Assert(err, IsNil)

	// 2 tikv on the same host with diffrent config style
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
	err = CheckTiKVLocationLabels([]string{"zone", "host"}, &topo)
	c.Assert(err, IsNil)
}
