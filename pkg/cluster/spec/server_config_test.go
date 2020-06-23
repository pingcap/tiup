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
