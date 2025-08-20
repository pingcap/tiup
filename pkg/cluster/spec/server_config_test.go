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
