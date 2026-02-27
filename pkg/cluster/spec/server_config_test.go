package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMerge(t *testing.T) {
	yamlData := []byte(`
server_configs:
  tidb:
    performance.feedback-probability: 12.0
    log.level: 0.0
    token-limit: 1000.1
`)

	topo := new(Specification)

	err := yaml.Unmarshal(yamlData, topo)
	require.NoError(t, err)

	// Verify values are parsed as float64
	require.Equal(t, float64(12.0), topo.ServerConfigs.TiDB["performance.feedback-probability"])
	require.Equal(t, float64(0.0), topo.ServerConfigs.TiDB["log.level"])
	require.Equal(t, float64(1000.1), topo.ServerConfigs.TiDB["token-limit"])

	// Verify Marshal/Unmarshal round-trip works without error
	_, err = yaml.Marshal(topo)
	require.NoError(t, err)

	get, err := Merge2Toml("tidb", topo.ServerConfigs.TiDB, nil)
	require.NoError(t, err)

	// Verify all float values retain decimal point in TOML output
	require.Contains(t, string(get), "12.0")
	require.Contains(t, string(get), "0.0")
	require.Contains(t, string(get), "1000.1")
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

func TestYAMLFloatSerialization(t *testing.T) {
	// Test that float values are serialized with decimal point preserved.
	// This ensures the forked yaml.v3 correctly handles float serialization.
	// See: https://github.com/go-yaml/yaml/issues/1038
	yamlData := []byte(`
server_configs:
  tidb:
    float_one: 1.0
    float_zero: 0.0
    float_value: 3.14
`)

	topo := new(Specification)
	err := yaml.Unmarshal(yamlData, topo)
	require.NoError(t, err)

	// Verify the values are correctly parsed as float64
	require.Equal(t, float64(1.0), topo.ServerConfigs.TiDB["float_one"])
	require.Equal(t, float64(0.0), topo.ServerConfigs.TiDB["float_zero"])
	require.Equal(t, float64(3.14), topo.ServerConfigs.TiDB["float_value"])

	// Marshal back to YAML
	marshaled, err := yaml.Marshal(topo)
	require.NoError(t, err)

	// The forked yaml.v3 should serialize float 1.0 as "1.0" (not "1")
	// This preserves the float type during round-trip serialization
	require.Contains(t, string(marshaled), "1.0")
	require.Contains(t, string(marshaled), "0.0")
	require.Contains(t, string(marshaled), "3.14")

	// Unmarshal again to verify type is preserved
	topo2 := new(Specification)
	err = yaml.Unmarshal(marshaled, topo2)
	require.NoError(t, err)

	// After round-trip, the values should still be float64
	require.IsType(t, float64(0), topo2.ServerConfigs.TiDB["float_one"])
	require.IsType(t, float64(0), topo2.ServerConfigs.TiDB["float_zero"])
	require.IsType(t, float64(0), topo2.ServerConfigs.TiDB["float_value"])
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
