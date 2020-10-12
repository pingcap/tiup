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
    a:
      b:
        c:
          d: 2
    a.b:
        c.e: 3
    a.b.c:
          f: 4
    h.i.j.k: [1, 2, 3]  
`)

	topo := new(Specification)

	err := yaml.Unmarshal(yamlData, topo)
	c.Assert(err, check.IsNil)

	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.d"), check.Equals, 1)
	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.e"), check.Equals, nil)
	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "a.b.c.f"), check.Equals, 4)
	c.Assert(GetValueFromPath(topo.ServerConfigs.TiDB, "h.i.j.k"), check.DeepEquals, []interface{}{1, 2, 3})
}
