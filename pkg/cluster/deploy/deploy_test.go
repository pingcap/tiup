package deploy

import (
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestVersionCompare(t *testing.T) {
	var err error

	err = versionCompare("v4.0.0", "v4.0.1")
	assert.Nil(t, err)

	err = versionCompare("v4.0.1", "v4.0.0")
	assert.NotNil(t, err)

	err = versionCompare("v4.0.0", "nightly")
	assert.Nil(t, err)

	err = versionCompare("nightly", "nightly")
	assert.Nil(t, err)
}

func TestValidateNewTopo(t *testing.T) {
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
	assert := require.New(t)
	assert.Nil(err)
	err = validateNewTopo(&topo)
	assert.Nil(err)

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
	assert.Nil(err)
	err = validateNewTopo(&topo)
	assert.NotNil(err)

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
	assert.Nil(err)
	err = validateNewTopo(&topo)
	assert.NotNil(err)
}
