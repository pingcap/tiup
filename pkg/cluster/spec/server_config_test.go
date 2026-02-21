package spec

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMerge(t *testing.T) {
	yamlData := []byte(`
server_configs:
  tidb:
    performance.feedback-probability: 12.0
`)

	topo := new(Specification)

	err := yaml.Unmarshal(yamlData, topo)
	require.NoError(t, err)

	yamlData, err = yaml.Marshal(topo)
	require.NoError(t, err)
	decimal := bytes.Contains(yamlData, []byte("12"))
	require.True(t, decimal)

	get, err := Merge2Toml("tidb", topo.ServerConfigs.TiDB, nil)
	require.NoError(t, err)

	decimal = bytes.Contains(get, []byte("12.0"))
	require.True(t, decimal)
}

func TestNGMonitoringServerConfig(t *testing.T) {
	yamlData := []byte(`
server_configs:
  ng_monitoring:
    storage.type: "sqlite"
    log.level: "WARN"
    continuous_profiling.enable: true
    continuous_profiling.profile_seconds: 5
    continuous_profiling.interval_seconds: 15

monitoring_servers:
  - host: 10.0.1.21
    ng_port: 12020
    ng_monitoring_config:
      storage.path: "/custom/data/path"
      continuous_profiling.data_retention_seconds: 259200
`)

	topo := new(Specification)
	err := yaml.Unmarshal(yamlData, topo)
	require.NoError(t, err)

	// Verify global config parsed
	require.Equal(t, "sqlite", topo.ServerConfigs.NGMonitoring["storage.type"])
	require.Equal(t, "WARN", topo.ServerConfigs.NGMonitoring["log.level"])
	require.Equal(t, true, topo.ServerConfigs.NGMonitoring["continuous_profiling.enable"])

	// Verify per-instance config parsed
	require.Len(t, topo.Monitors, 1)
	require.Equal(t, "/custom/data/path", topo.Monitors[0].NgMonitoringConfig["storage.path"])

	// Build base config (simulating what InitConfig does)
	baseConfig := map[string]any{
		"address":           "0.0.0.0:12020",
		"advertise-address": "10.0.1.21:12020",
		"log.path":          "/tidb-deploy/prometheus-9090/log",
		"log.level":         "INFO",
		"pd.endpoints":      []string{"10.0.1.10:2379"},
		"storage.path":      "/tidb-data/prometheus-9090",
	}

	// Merge: base + global + per-instance (same as InitConfig logic)
	userConfig := MergeConfig(topo.ServerConfigs.NGMonitoring, topo.Monitors[0].NgMonitoringConfig)
	got, err := Merge2Toml("ng_monitoring", baseConfig, userConfig)
	require.NoError(t, err)

	tomlStr := string(got)

	// User overrides should take precedence
	require.Contains(t, tomlStr, `type = "sqlite"`)                 // from global server_configs
	require.Contains(t, tomlStr, `path = "/custom/data/path"`)      // per-instance overrides base
	require.Contains(t, tomlStr, `level = "WARN"`)                  // global overrides default
	require.Contains(t, tomlStr, `enable = true`)                   // from global
	require.Contains(t, tomlStr, `data_retention_seconds = 259200`) // from per-instance
	require.Contains(t, tomlStr, `address = "0.0.0.0:12020"`)       // from base
}

func TestGetValueFromPath(t *testing.T) {
	yamlData := []byte(`
server_configs:
  tidb:
    a.b.c.d: 1
    a.b:
        c.e: 3
    a.b.c:
          f: 4
    h.i.j.k: [1, 2, 4]
    e:
      f: true
`)

	topo := new(Specification)

	err := yaml.Unmarshal(yamlData, topo)
	require.NoError(t, err)

	require.Equal(t, 1, GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.d"))
	require.Equal(t, 3, GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.e"))
	require.Equal(t, 4, GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.f"))
	require.Equal(t, []any{1, 2, 4}, GetValueFromPath(topo.ServerConfigs.TiDB, "h.i.j.k"))
	require.Equal(t, true, GetValueFromPath(topo.ServerConfigs.TiDB, "e.f"))
}

func TestFlattenMap(t *testing.T) {
	var (
		m map[string]any
		r map[string]any
	)

	m = map[string]any{
		"a": 1,
		"b": map[string]any{
			"c": 2,
		},
		"d.e": 3,
		"f.g": map[string]any{
			"h": 4,
			"i": 5,
		},
		"j": []int{6, 7},
	}
	r = FlattenMap(m)
	require.Equal(t, 1, r["a"])
	require.Equal(t, 2, r["b.c"])
	require.Equal(t, 3, r["d.e"])
	require.Equal(t, 4, r["f.g.h"])
	require.Equal(t, 5, r["f.g.i"])
	require.Equal(t, []int{6, 7}, r["j"])
}

func TestFoldMap(t *testing.T) {
	var (
		m map[string]any
		r map[string]any
	)

	m = map[string]any{
		"a":   1,
		"b.c": 2,
		"b.d": 3,
		"e.f": map[string]any{
			"g.h": 4,
		},
		"i": map[string]any{
			"j.k": 5,
			"l":   6,
		},
	}

	r = FoldMap(m)
	require.Equal(t, map[string]any{
		"a": 1,
		"b": map[string]any{
			"c": 2,
			"d": 3,
		},
		"e": map[string]any{
			"f": map[string]any{
				"g": map[string]any{
					"h": 4,
				},
			},
		},
		"i": map[string]any{
			"j": map[string]any{
				"k": 5,
			},
			"l": 6,
		},
	}, r)
}

func TestEncodeRemoteCfg(t *testing.T) {
	yamlData := []byte(`remote_write:
    - queue_config:
        batch_send_deadline: 5m
        capacity: 100000
        max_samples_per_send: 10000
        max_shards: 300
      url: http://127.0.0.1:/8086/write
remote_read:
    - url: http://127.0.0.1:/8086/read
    - url: http://127.0.0.1:/8087/read
`)

	bs, err := encodeRemoteCfg2Yaml(Remote{
		RemoteWrite: []map[string]any{
			{
				"url": "http://127.0.0.1:/8086/write",
				"queue_config": map[string]any{
					"batch_send_deadline":  "5m",
					"capacity":             100000,
					"max_samples_per_send": 10000,
					"max_shards":           300,
				},
			},
		},
		RemoteRead: []map[string]any{
			{
				"url": "http://127.0.0.1:/8086/read",
			},
			{
				"url": "http://127.0.0.1:/8087/read",
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, yamlData, bs)
}
