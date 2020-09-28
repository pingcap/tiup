package spec

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/check"
)

func TestUtils(t *testing.T) {
	check.TestingT(t)
}

type topoSuite struct{}

var _ = check.Suite(&topoSuite{})

func withTempFile(content string, fn func(string)) {
	file, err := ioutil.TempFile("/tmp", "topology-test")
	if err != nil {
		panic(fmt.Sprintf("create temp file: %s", err))
	}
	defer os.Remove(file.Name())

	_, err = file.WriteString(content)
	if err != nil {
		panic(fmt.Sprintf("write temp file: %s", err))
	}
	file.Close()

	fn(file.Name())
}

func (s *topoSuite) TestParseTopologyYaml(c *check.C) {
	file := filepath.Join("testdata", "topology_err.yaml")
	topo := Specification{}
	err := ParseTopologyYaml(file, &topo)
	c.Assert(err, check.IsNil)
	FixRelativeDir(&topo)

	// test relative path
	withTempFile(`
tikv_servers:
  - host: 172.16.5.140
    deploy_dir: my-deploy
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)
		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")
	})

	// test data dir & log dir
	withTempFile(`
tikv_servers:
  - host: 172.16.5.140
    deploy_dir: my-deploy
    data_dir: my-data
    log_dir: my-log
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)
		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/my-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/my-deploy/my-log")
	})

	// test global options, case 1
	withTempFile(`
global:
  deploy_dir: my-deploy

tikv_servers:
  - host: 172.16.5.140
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "/home/tidb/my-deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "/home/tidb/my-deploy/data")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/data")
	})

	// test global options, case 2
	withTempFile(`
global:
  deploy_dir: my-deploy

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "/home/tidb/my-deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "/home/tidb/my-deploy/data")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/data")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20161/data")
	})

	// test global options, case 3
	withTempFile(`
global:
  deploy_dir: my-deploy

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    data_dir: my-data
    log_dir: my-log
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "/home/tidb/my-deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "/home/tidb/my-deploy/data")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/my-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/my-deploy/tikv-20160/my-log")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/tidb/my-deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/tidb/my-deploy/tikv-20161/data")
		c.Assert(topo.TiKVServers[1].LogDir, check.Equals, "")
	})

	// test global options, case 4
	withTempFile(`
global:
  data_dir: my-global-data
  log_dir: my-global-log

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    data_dir: my-local-data
    log_dir: my-local-log
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "/home/tidb/deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "/home/tidb/deploy/my-global-data")
		c.Assert(topo.GlobalOptions.LogDir, check.Equals, "/home/tidb/deploy/my-global-log")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tikv-20160")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/deploy/tikv-20160/my-local-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/deploy/tikv-20160/my-local-log")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/tidb/deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/tidb/deploy/tikv-20161/my-global-data")
		c.Assert(topo.TiKVServers[1].LogDir, check.Equals, "/home/tidb/deploy/tikv-20161/my-global-log")
	})

	// test multiple dir, case 5
	withTempFile(`
tiflash_servers:
  - host: 172.16.5.140
    data_dir: /path/to/my-first-data,my-second-data
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)

		c.Assert(topo.TiFlashServers[0].DeployDir, check.Equals, "/home/tidb/deploy/tiflash-9000")
		c.Assert(topo.TiFlashServers[0].DataDir, check.Equals, "/path/to/my-first-data,/home/tidb/deploy/tiflash-9000/my-second-data")
		c.Assert(topo.TiFlashServers[0].LogDir, check.Equals, "")
	})

	// test global options, case 6
	withTempFile(`
global:
  data_dir: my-global-data
  log_dir: my-global-log

tikv_servers:
  - host: 172.16.5.140
    port: 20160
    status_port: 20180
    deploy_dir: my-local-deploy
    data_dir: my-local-data
    log_dir: my-local-log
  - host: 172.16.5.140
    port: 20161
    status_port: 20181
`, func(file string) {
		topo := Specification{}
		err := ParseTopologyYaml(file, &topo)
		c.Assert(err, check.IsNil)
		FixRelativeDir(&topo)
		c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "/home/tidb/deploy")
		c.Assert(topo.GlobalOptions.DataDir, check.Equals, "/home/tidb/deploy/my-global-data")
		c.Assert(topo.GlobalOptions.LogDir, check.Equals, "/home/tidb/deploy/my-global-log")

		c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-local-deploy")
		c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-local-deploy/my-local-data")
		c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/my-local-deploy/my-local-log")

		c.Assert(topo.TiKVServers[1].DeployDir, check.Equals, "/home/tidb/deploy/tikv-20161")
		c.Assert(topo.TiKVServers[1].DataDir, check.Equals, "/home/tidb/deploy/tikv-20161/my-global-data")
		c.Assert(topo.TiKVServers[1].LogDir, check.Equals, "/home/tidb/deploy/tikv-20161/my-global-log")
	})
}

func (s *topoSuite) TestFixRelativePath(c *check.C) {
	// base test
	topo := Specification{
		TiKVServers: []TiKVSpec{
			{
				DeployDir: "my-deploy",
			},
		},
	}
	fixRelativePath("tidb", &topo)
	c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")

	// test data dir & log dir
	topo = Specification{
		TiKVServers: []TiKVSpec{
			{
				DeployDir: "my-deploy",
				DataDir:   "my-data",
				LogDir:    "my-log",
			},
		},
	}
	fixRelativePath("tidb", &topo)
	c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "/home/tidb/my-deploy")
	c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "/home/tidb/my-deploy/my-data")
	c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "/home/tidb/my-deploy/my-log")

	// test global options
	topo = Specification{
		GlobalOptions: GlobalOptions{
			DeployDir: "my-deploy",
			DataDir:   "my-data",
			LogDir:    "my-log",
		},
		TiKVServers: []TiKVSpec{
			{},
		},
	}
	fixRelativePath("tidb", &topo)
	c.Assert(topo.GlobalOptions.DeployDir, check.Equals, "/home/tidb/my-deploy")
	c.Assert(topo.GlobalOptions.DataDir, check.Equals, "/home/tidb/my-deploy/my-data")
	c.Assert(topo.GlobalOptions.LogDir, check.Equals, "/home/tidb/my-deploy/my-log")
	c.Assert(topo.TiKVServers[0].DeployDir, check.Equals, "")
	c.Assert(topo.TiKVServers[0].DataDir, check.Equals, "")
	c.Assert(topo.TiKVServers[0].LogDir, check.Equals, "")
}
