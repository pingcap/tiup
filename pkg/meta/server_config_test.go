package meta

import (
	"bytes"

	goyaml "github.com/goccy/go-yaml"
	"github.com/pingcap/check"
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

	topo := new(TopologySpecification)

	err := goyaml.Unmarshal(yamlData, topo)
	c.Assert(err, check.IsNil)

	yamlData, err = goyaml.Marshal(topo)
	c.Assert(err, check.IsNil)
	decimal := bytes.Contains(yamlData, []byte("0.0"))
	c.Assert(decimal, check.IsTrue)

	get, err := merge2Toml("tidb", topo.ServerConfigs.TiDB, nil)
	c.Assert(err, check.IsNil)

	decimal = bytes.Contains(get, []byte("0.0"))
	c.Assert(decimal, check.IsTrue)
}
