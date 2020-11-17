package spec

import (
	"bytes"

	"github.com/pingcap/check"
	"gopkg.in/yaml.v2"
)

type configSuite struct {
}

var _ = check.Suite(&configSuite{})

func (s *configSuite) TestMerge(c *check.C) {
	yamlData := []byte(`
server_configs:
  tidb:
    performance.feedback-probability: 0.0
`)

	topo := new(Specification)

	err := yaml.Unmarshal(yamlData, topo)
	c.Assert(err, check.IsNil)

	yamlData, err = yaml.Marshal(topo)
	c.Assert(err, check.IsNil)
	decimal := bytes.Contains(yamlData, []byte("0.0"))
	c.Assert(decimal, check.IsTrue)

	get, err := merge2Toml("tidb", topo.ServerConfigs.TiDB, nil)
	c.Assert(err, check.IsNil)

	decimal = bytes.Contains(get, []byte("0.0"))
	c.Assert(decimal, check.IsTrue)
}

func (s *configSuite) TestGetValueFromPath(c *check.C) {
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
	c.Assert(err, check.IsNil)

	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.d"), check.Equals, 1)
	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.e"), check.Equals, 3)
	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.f"), check.Equals, 4)
	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "h.i.j.k"), check.DeepEquals, []interface{}{1, 2, 4})
	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "e.f"), check.Equals, true)
}

func (s *configSuite) TestFlattenMap(c *check.C) {
	var (
		m map[string]interface{}
		r map[string]interface{}
	)

	m = map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": 2,
		},
		"d.e": 3,
		"f.g": map[string]interface{}{
			"h": 4,
			"i": 5,
		},
		"j": []int{6, 7},
	}
	r = FlattenMap(m)
	c.Assert(r["a"], check.Equals, 1)
	c.Assert(r["b.c"], check.Equals, 2)
	c.Assert(r["d.e"], check.Equals, 3)
	c.Assert(r["f.g.h"], check.Equals, 4)
	c.Assert(r["f.g.i"], check.Equals, 5)
	c.Assert(r["j"], check.DeepEquals, []int{6, 7})
}

func (s *configSuite) TestFoldMap(c *check.C) {
	var (
		m map[string]interface{}
		r map[string]interface{}
	)

	m = map[string]interface{}{
		"a":   1,
		"b.c": 2,
		"b.d": 3,
		"e.f": map[string]interface{}{
			"g.h": 4,
		},
		"i": map[string]interface{}{
			"j.k": 5,
			"l":   6,
		},
	}

	r = FoldMap(m)
	c.Assert(r, check.DeepEquals, map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": 2,
			"d": 3,
		},
		"e": map[string]interface{}{
			"f": map[string]interface{}{
				"g": map[string]interface{}{
					"h": 4,
				},
			},
		},
		"i": map[string]interface{}{
			"j": map[string]interface{}{
				"k": 5,
			},
			"l": 6,
		},
	})
}
