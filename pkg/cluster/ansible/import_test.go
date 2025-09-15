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
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMonitoredDeployDir(t *testing.T) {
	r := strings.NewReader(`
[monitored_servers]
172.16.10.1
172.16.10.2
172.16.10.3

[all:vars]
process_supervision = systemd
	`)

	_, clsMeta, _, err := parseInventoryFile(r)
	require.NoError(t, err)
	require.Equal(t, "", clsMeta.Topology.MonitoredOptions.DeployDir)

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
	require.NoError(t, err)
	require.Equal(t, "/data1/deploy", clsMeta.Topology.MonitoredOptions.DeployDir)

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
	require.NoError(t, err)
	require.Equal(t, "/data/deploy", clsMeta.Topology.MonitoredOptions.DeployDir)

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
	require.NoError(t, err)
	require.Equal(t, "/data1/deploy", clsMeta.Topology.MonitoredOptions.DeployDir)
}

func TestParseInventoryFile(t *testing.T) {
	dir := "test-data"
	invData, err := os.Open(filepath.Join(dir, "inventory.ini"))
	require.NoError(t, err)

	clsName, clsMeta, inv, err := parseInventoryFile(invData)
	require.NoError(t, err)
	require.NotNil(t, inv)
	require.Equal(t, "ansible-cluster", clsName)
	require.NotNil(t, clsMeta)
	require.Equal(t, "v3.0.12", clsMeta.Version)
	require.Equal(t, "tiops", clsMeta.User)

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
    tso: {}
    scheduling: {}
    tidb_dashboard: {}
    tiflash: {}
    tiproxy: {}
    tiflash-learner: {}
    pump: {}
    drainer: {}
    cdc: {}
    kvcdc: {}
    tici_meta: {}
    tici_worker: {}
    grafana: {}
tidb_servers: []
tikv_servers: []
tiflash_servers: []
tiproxy_servers: []
pd_servers: []
monitoring_servers: []
`)

	topo, err := yaml.Marshal(clsMeta.Topology)
	require.NoError(t, err)
	require.Equal(t, string(expected), string(topo))
}

func TestParseGroupVars(t *testing.T) {
	dir := "test-data"
	ansCfgFile := filepath.Join(dir, "ansible.cfg")
	invData, err := os.Open(filepath.Join(dir, "inventory.ini"))
	require.NoError(t, err)
	_, clsMeta, inv, err := parseInventoryFile(invData)
	require.NoError(t, err)

	err = parseGroupVars(context.WithValue(
		context.TODO(), logprinter.ContextKeyLogger, logprinter.NewLogger(""),
	), dir, ansCfgFile, clsMeta, inv)
	require.NoError(t, err)
	err = defaults.Set(clsMeta)
	require.NoError(t, err)

	var expected spec.ClusterMeta
	var metaFull spec.ClusterMeta

	expectedTopo, err := os.ReadFile(filepath.Join(dir, "meta.yaml"))
	require.NoError(t, err)
	err = yaml.Unmarshal(expectedTopo, &expected)
	require.NoError(t, err)

	// marshal and unmarshal the meta to ensure custom defaults are populated
	meta, err := yaml.Marshal(clsMeta)
	require.NoError(t, err)
	err = yaml.Unmarshal(meta, &metaFull)
	require.NoError(t, err)

	sortClusterMeta(&metaFull)
	sortClusterMeta(&expected)

	_, err = yaml.Marshal(metaFull)
	require.NoError(t, err)
	require.Equal(t, expected, metaFull)
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

func TestParseConfig(t *testing.T) {
	// base test
	withTempFile(`
a = true

[b]
c = 1
d = "\""
`, func(file string) {
		m, err := parseConfigFile(file)
		require.NoError(t, err)
		require.Nil(t, m["x"])
		require.Equal(t, true, m["a"])
		require.Equal(t, int64(1), m["b.c"])
		require.Equal(t, "\"", m["b.d"])
	})
}

func TestDiffConfig(t *testing.T) {
	global, locals := diffConfigs([]map[string]any{
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

	require.NotNil(t, global["a"])
	require.Nil(t, global["b"])
	require.Equal(t, true, global["a"])
	require.Equal(t, 1, global["foo.bar"])
	require.Equal(t, 1, locals[0]["b"])
	require.Equal(t, 2, locals[1]["b"])
	require.Equal(t, 3, locals[2]["b"])
}
