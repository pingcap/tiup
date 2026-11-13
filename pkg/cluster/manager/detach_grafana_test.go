// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func newGrafanaDetachTestManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("TIUP_COMPONENT_DATA_DIR", t.TempDir())
	require.NoError(t, spec.Initialize("cluster"))
	return NewManager("tidb", spec.GetSpecManager(), logprinter.NewLogger(""))
}

func grafanaDetachTestMetadata(t *testing.T) *spec.ClusterMeta {
	t.Helper()
	metadata := &spec.ClusterMeta{User: "tidb", Version: "v8.5.6", Topology: &spec.Specification{}}
	require.NoError(t, yaml.Unmarshal([]byte(`
global:
  user: tidb
component_versions:
  grafana: v8.5.6
server_configs:
  tidb:
    log.level: warn
pd_servers:
  - host: n1
  - host: n3
  - host: n4
tidb_servers:
  - host: n1
tikv_servers:
  - host: n1
tiflash_servers:
  - host: n3
monitoring_servers:
  - host: n1
grafana_servers:
  - host: n1
    port: 3000
    patched: true
    config:
      log.file.level: debug
  - host: n2
    port: 3001
    patched: true
`), metadata.Topology))
	return metadata
}

func TestDetachGrafanaPreservesMetadata(t *testing.T) {
	for _, tc := range []struct {
		name  string
		nodes []string
		keep  int
	}{
		{"subset and duplicates", []string{" n1:3000 ", "n1:3000"}, 1},
		{"all", []string{"n1:3000", "n2:3001"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newGrafanaDetachTestManager(t)
			require.NoError(t, spec.SaveClusterMeta("test", grafanaDetachTestMetadata(t)))
			before, err := spec.ClusterMetadata("test")
			require.NoError(t, err)
			original, err := os.ReadFile(m.specManager.Path("test", "meta.yaml"))
			require.NoError(t, err)
			want := *before
			wantTopology := *before.Topology
			want.Topology = &wantTopology
			want.Topology.Grafanas = before.Topology.Grafanas[len(before.Topology.Grafanas)-tc.keep:]
			if tc.keep == 0 {
				want.Topology.Grafanas = nil
			}

			require.NoError(t, m.DetachGrafana("test", tc.nodes, true))
			after, err := spec.ClusterMetadata("test")
			require.NoError(t, err)
			require.Equal(t, &want, after)
			backups, err := os.ReadDir(m.specManager.Path("test", spec.BackupDirName))
			require.NoError(t, err)
			require.Len(t, backups, 1)
			backup, err := os.ReadFile(m.specManager.Path("test", spec.BackupDirName, backups[0].Name()))
			require.NoError(t, err)
			require.Equal(t, original, backup)
		})
	}
}

func TestDetachGrafanaRejectsWithoutSaving(t *testing.T) {
	for _, tc := range []struct {
		name   string
		nodes  []string
		locked bool
		other  string
		want   string
	}{
		{name: "no nodes", want: "at least one"},
		{name: "empty ID", nodes: []string{"n1:3000", " "}, want: "must not be empty"},
		{name: "unknown ID", nodes: []string{"n1:3000", "missing:3000"}, want: "does not exist"},
		{name: "mixed roles", nodes: []string{"n1:3000", "n1:2379"}, want: "not Grafana"},
		{name: "stage1", nodes: []string{"n1:3000"}, locked: true, want: "scale-out"},
		{name: "unreadable other metadata", nodes: []string{"n1:3000"}, other: "topology: [", want: "cluster \"other\""},
		{name: "missing other topology", nodes: []string{"n1:3000"}, other: "user: tidb\n", want: "has no topology"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newGrafanaDetachTestManager(t)
			require.NoError(t, spec.SaveClusterMeta("test", grafanaDetachTestMetadata(t)))
			if tc.locked {
				require.NoError(t, os.WriteFile(m.specManager.Path("test", spec.ScaleOutLockName), []byte("{}"), 0o600))
			}
			if tc.other != "" {
				require.NoError(t, os.MkdirAll(m.specManager.Path("other"), 0o700))
				require.NoError(t, os.WriteFile(m.specManager.Path("other", "meta.yaml"), []byte(tc.other), 0o600))
			}
			before, err := os.ReadFile(m.specManager.Path("test", "meta.yaml"))
			require.NoError(t, err)
			require.ErrorContains(t, m.DetachGrafana("test", tc.nodes, true), tc.want)
			after, err := os.ReadFile(m.specManager.Path("test", "meta.yaml"))
			require.NoError(t, err)
			require.Equal(t, before, after)
			backups, err := os.ReadDir(m.specManager.Path("test", spec.BackupDirName))
			require.NoError(t, err)
			require.Empty(t, backups)
		})
	}
}

func TestDetachGrafanaDirectorySafety(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*spec.ClusterMeta)
		other  bool
		want   string
	}{
		{name: "default shared host and sibling directories"},
		{name: "Grafana nested in PD", modify: func(m *spec.ClusterMeta) {
			m.Topology.Grafanas[0].DeployDir = "/home/tidb/deploy/pd-2379/grafana-3000"
		}, want: "overlaps"},
		{name: "managed directory nested in Grafana", modify: func(m *spec.ClusterMeta) {
			m.Topology.TiDBServers[0].DeployDir = "/home/tidb/deploy/grafana-3000/tidb"
		}, want: "overlaps"},
		{name: "managed external log", modify: func(m *spec.ClusterMeta) {
			m.Topology.TiDBServers[0].LogDir = "/home/tidb/deploy/grafana-3000/tidb-logs"
		}, want: "overlaps"},
		{name: "multi data directories", modify: func(m *spec.ClusterMeta) {
			m.Topology.TiKVServers[0].DataDir = "/var/lib/tikv,/home/tidb/deploy/grafana-3000/kv-data"
		}, want: "overlaps"},
		{name: "monitored directory", modify: func(m *spec.ClusterMeta) {
			m.Topology.MonitoredOptions.DeployDir = "/home/tidb/deploy/grafana-3000/monitor"
		}, want: "monitored deploy"},
		{name: "relative monitored data cleanup", modify: func(m *spec.ClusterMeta) {
			m.Topology.Grafanas[0].DeployDir = "/home/tidb/data/monitor-9100/grafana-3000"
		}, want: "monitored data cleanup"},
		{name: "TiFlash external cache", modify: func(m *spec.ClusterMeta) {
			m.Topology.TiFlashServers[0].ManageHost = "n1"
			m.Topology.TiFlashServers[0].Config = map[string]any{spec.TiFlashRemoteCacheDir: "/home/tidb/deploy/grafana-3000/cache"}
		}, want: spec.TiFlashRemoteCacheDir},
		{name: "global Grafana external data", modify: func(m *spec.ClusterMeta) {
			m.Topology.ServerConfigs.Grafana = map[string]string{"paths.data": "/home/tidb/deploy/pd-2379/shared"}
		}, want: "paths.data"},
		{name: "instance path overrides global", modify: func(m *spec.ClusterMeta) {
			m.Topology.ServerConfigs.Grafana = map[string]string{"paths.data": "/home/tidb/deploy/pd-2379/shared"}
			m.Topology.Grafanas[0].Config["paths.data"] = "/srv/independent-grafana-data"
		}},
		{name: "relative override", modify: func(m *spec.ClusterMeta) {
			m.Topology.Grafanas[0].Config["paths.data"] = "data"
		}, want: "absolute literal path"},
		{name: "managed shell glob", modify: func(m *spec.ClusterMeta) {
			m.Topology.PDServers[0].DeployDir = "/srv/pd*"
			m.Topology.Grafanas[0].DeployDir = "/srv/pd-grafana"
		}, want: "absolute literal path"},
		{name: "managed shell whitespace", modify: func(m *spec.ClusterMeta) {
			m.Topology.PDServers[0].DeployDir = "/srv/pd /srv/grafana"
			m.Topology.Grafanas[0].DeployDir = "/srv/grafana"
		}, want: "absolute literal path"},
		{name: "empty override", modify: func(m *spec.ClusterMeta) {
			m.Topology.Grafanas[0].Config["paths.logs"] = ""
		}, want: "absolute literal path"},
		{name: "environment provider", modify: func(m *spec.ClusterMeta) {
			m.Topology.Grafanas[0].Config["paths.plugins"] = "/srv/$__env{PLUGIN_DIR}"
		}, want: "absolute literal path"},
		{name: "different host does not block", modify: func(m *spec.ClusterMeta) {
			m.Topology.Grafanas[1].Config = map[string]string{"paths.data": "$__env{DATA_DIR}"}
		}},
		{name: "same manage host validates other Grafana paths", modify: func(m *spec.ClusterMeta) {
			m.Topology.Grafanas[1].ManageHost = "n1"
			m.Topology.Grafanas[1].Config = map[string]string{"paths.data": "$__env{DATA_DIR}"}
		}, want: "absolute literal path"},
		{name: "path prefix is not overlap", modify: func(m *spec.ClusterMeta) {
			m.Topology.TiDBServers[0].DeployDir = "/home/tidb/deploy/grafana-3000-other"
		}},
		{name: "cross cluster parent", other: true, modify: func(m *spec.ClusterMeta) {
			m.Topology.PDServers[0].DeployDir = "/home/tidb/deploy"
		}, want: "cluster \"other\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newGrafanaDetachTestManager(t)
			metadata := grafanaDetachTestMetadata(t)
			if tc.other {
				other := grafanaDetachTestMetadata(t)
				tc.modify(other)
				require.NoError(t, spec.SaveClusterMeta("other", other))
			} else if tc.modify != nil {
				tc.modify(metadata)
			}
			require.NoError(t, spec.SaveClusterMeta("test", metadata))
			before, err := os.ReadFile(m.specManager.Path("test", "meta.yaml"))
			require.NoError(t, err)
			err = m.DetachGrafana("test", []string{"n1:3000"}, true)
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.want)
			after, err := os.ReadFile(m.specManager.Path("test", "meta.yaml"))
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestGrafanaDetachPathsOverlap(t *testing.T) {
	for _, tc := range []struct {
		first, second string
		overlap       bool
	}{
		{"/srv/a", "/srv/a", true},
		{"/srv/a", "/srv/a/data", true},
		{"/srv/a/data", "/srv/a", true},
		{"/srv/a", "/srv/ab", false},
		{"/srv/a/../b", "/srv/b/data", true},
		{"/", "/srv/a", true},
	} {
		require.Equal(t, tc.overlap, grafanaDetachPathsOverlap(tc.first, tc.second), "%s, %s", tc.first, tc.second)
	}
}

func TestDetachGrafanaMissingTopology(t *testing.T) {
	m := newGrafanaDetachTestManager(t)
	require.NoError(t, os.MkdirAll(m.specManager.Path("test"), 0o700))
	path := filepath.Join(m.specManager.Path("test"), "meta.yaml")
	require.NoError(t, os.WriteFile(path, []byte("user: tidb\ntopology: null\n"), 0o600))
	require.ErrorContains(t, m.DetachGrafana("test", []string{"n1:3000"}, true), "has no topology")
}
