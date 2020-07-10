package command

import (
	"github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"gopkg.in/yaml.v2"
)

type scaleOutSuite struct{}

var _ = check.Suite(&scaleOutSuite{})

func (s *scaleOutSuite) TestValidateNewTopo(c *check.C) {
	topo := spec.Specification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data" 
tidb_servers:
  - host: 172.16.5.138
    deploy_dir: "tidb-deploy"
pd_servers:
  - host: 172.16.5.53
    data_dir: "pd-data"
`), &topo)
	c.Assert(err, check.IsNil)
	err = validateNewTopo(&topo)
	c.Assert(err, check.IsNil)

	topo = spec.Specification{}
	err = yaml.Unmarshal([]byte(`
tidb_servers:
  - host: 172.16.5.138
    imported: true
    deploy_dir: "tidb-deploy"
pd_servers:
  - host: 172.16.5.53
    data_dir: "pd-data"
`), &topo)
	c.Assert(err, check.IsNil)
	err = validateNewTopo(&topo)
	c.Assert(err, check.NotNil)

	topo = spec.Specification{}
	err = yaml.Unmarshal([]byte(`
global:
  user: "test3"
  deploy_dir: "test-deploy"
  data_dir: "test-data"
pd_servers:
  - host: 172.16.5.53
    imported: true
`), &topo)
	c.Assert(err, check.IsNil)
	err = validateNewTopo(&topo)
	c.Assert(err, check.NotNil)
}
