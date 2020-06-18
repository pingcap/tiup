package meta

import (
	"testing"

	. "github.com/pingcap/check"
	"gopkg.in/yaml.v2"
)

type metaSuite struct {
}

var _ = Suite(&metaSuite{})

func TestMeta(t *testing.T) {
	TestingT(t)
}

func (s *metaSuite) TestDefaultDataDir(c *C) {
	// Test with without global DataDir.
	topo := new(DMSTopologySpecification)
	topo.Job.Action = "import"
	topo.Job.Workers = append(topo.Job.Workers, WorkerSpec{Host: "1.1.1.1", SSHPort: 22})
	data, err := yaml.Marshal(topo)
	c.Assert(err, IsNil)

	// Check default value.
	topo = new(DMSTopologySpecification)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "data")
	c.Assert(topo.Job.Workers[0].DataDir, Equals, "data")

	// Can keep the default value.
	data, err = yaml.Marshal(topo)
	c.Assert(err, IsNil)
	topo = new(DMSTopologySpecification)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "data")
	c.Assert(topo.Job.Workers[0].DataDir, Equals, "data")

	// Test with global DataDir.
	topo = new(DMSTopologySpecification)
	topo.GlobalOptions.DataDir = "/global_data"
	topo.Job.Action = "import"
	topo.Job.Workers = append(topo.Job.Workers, WorkerSpec{Host: "1.1.1.1", SSHPort: 22})
	topo.Job.Workers = append(topo.Job.Workers, WorkerSpec{Host: "1.1.1.2", SSHPort: 33, DataDir: "/my_data"})
	data, err = yaml.Marshal(topo)
	c.Assert(err, IsNil)

	topo = new(DMSTopologySpecification)
	err = yaml.Unmarshal(data, topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.DataDir, Equals, "/global_data")
	c.Assert(topo.Job.Workers[0].DataDir, Equals, "/global_data")
	c.Assert(topo.Job.Workers[1].DataDir, Equals, "/my_data")
}

func (s *metaSuite) TestGlobalOptions(c *C) {
	topo := DMSTopologySpecification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data" 
job:
  action: import
  type: csv
  workers:
  - host: 172.16.5.138
    deploy_dir: "worker-deploy"
  - host: 172.16.5.53
    data_dir: "worker-data"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.GlobalOptions.User, Equals, "test1")
	c.Assert(topo.GlobalOptions.SSHPort, Equals, 220)
	c.Assert(topo.Job.Workers[0].SSHPort, Equals, 220)
	c.Assert(topo.Job.Workers[0].DeployDir, Equals, "worker-deploy")
	c.Assert(topo.Job.Workers[0].DataDir, Equals, "test-data")

	c.Assert(topo.Job.Workers[1].SSHPort, Equals, 220)
	c.Assert(topo.Job.Workers[1].DeployDir, Equals, "test-deploy")
	c.Assert(topo.Job.Workers[1].DataDir, Equals, "worker-data")
}

func (s *metaSuite) TestDataDirAbsolute(c *C) {
	topo := DMSTopologySpecification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  data_dir: "/test-data" 
job:
  action: import
  type: csv
  workers:
  - host: 172.16.5.53
    data_dir: "worker-data"
  - host: 172.16.5.54
`), &topo)
	c.Assert(err, IsNil)

	c.Assert(topo.Job.Workers[0].DataDir, Equals, "worker-data")
	c.Assert(topo.Job.Workers[1].DataDir, Equals, "/test-data")
}

func (s *metaSuite) TestDirectoryConflicts(c *C) {
	topo := DMSTopologySpecification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data" 
monitoring_servers:
  - host: 172.16.5.138
    deploy_dir: "/test-1"
alertmanager_servers:
  - host: 172.16.5.138
    data_dir: "/test-1"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "directory '/test-1' conflicts between 'monitoring_servers:172.16.5.138.deploy_dir' and 'alertmanager_servers,omitempty:172.16.5.138.data_dir'")

	err = yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "/test-data" 
monitoring_servers:
  - host: 172.16.5.138
    data_dir: "test-1"
alertmanager_servers:
  - host: 172.16.5.138
    data_dir: "test-1"
`), &topo)
	c.Assert(err, IsNil)
}

func (s *metaSuite) TestPortConflicts(c *C) {
	topo := DMSTopologySpecification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  ssh_port: 220
  deploy_dir: "test-deploy"
  data_dir: "test-data" 
monitoring_servers:
  - host: 172.16.5.138
    port: 1234
alertmanager_servers:
  - host: 172.16.5.138
    web_port: 1234
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "port '1234' conflicts between 'monitoring_servers:172.16.5.138.port' and 'alertmanager_servers,omitempty:172.16.5.138.web_port'")

	topo = DMSTopologySpecification{}
	err = yaml.Unmarshal([]byte(`
monitored:
  node_exporter_port: 1234
monitoring_servers:
  - host: 172.16.5.138
    port: 1234
alertmanager_servers:
  - host: 172.16.5.138
    web_port: 2345
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "port '1234' conflicts between 'monitoring_servers:172.16.5.138.port' and 'monitored:172.16.5.138.node_exporter_port'")
}

func (s *metaSuite) TestPlatformConflicts(c *C) {
	// aarch64 and arm64 are equal
	topo := DMSTopologySpecification{}
	err := yaml.Unmarshal([]byte(`
global:
  os: "linux"
  arch: "aarch64"
monitoring_servers:
  - host: 172.16.5.138
    arch: "arm64"
alertmanager_servers:
  - host: 172.16.5.138
`), &topo)
	c.Assert(err, IsNil)

	// different arch defined for the same host
	topo = DMSTopologySpecification{}
	err = yaml.Unmarshal([]byte(`
global:
  os: "linux"
job:
  action: import
  type: csv
  workers:
  - host: 172.16.5.138
    data_dir: "worker-data"
  - host: 172.16.5.138
    arch: "aarch64"
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "platform mismatch for '172.16.5.138' as in 'workers:linux/amd64' and 'workers:linux/arm64'")

	// different os defined for the same host
	topo = DMSTopologySpecification{}
	err = yaml.Unmarshal([]byte(`
global:
  os: "linux"
job:
  action: import
  type: csv
  workers:
  - host: 172.16.5.138
    os: "darwin"
    data_dir: "worker-data"
  - host: 172.16.5.138
`), &topo)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "platform mismatch for '172.16.5.138' as in 'workers:darwin/amd64' and 'workers:linux/amd64'")

}

func (s *metaSuite) TestLogDir(c *C) {
	topo := DMSTopologySpecification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  data_dir: "/test-data" 
job:
  action: import
  type: csv
  workers:
  - host: 172.16.5.53
    log_dir: test-deploy/log
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.Job.Workers[0].LogDir, Equals, "test-deploy/log")
}

func (s *metaSuite) TestMonitorLogDir(c *C) {
	topo := DMSTopologySpecification{}
	err := yaml.Unmarshal([]byte(`
global:
  user: "test1"
  data_dir: "/test-data" 
job:
  action: import
  type: csv
  workers:
  - host: 172.16.5.53
    log_dir: test-deploy/log
monitored:
    deploy_dir: "test-deploy"
    log_dir: "test-deploy/log"
`), &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.MonitoredOptions.LogDir, Equals, "test-deploy/log")

	out, err := yaml.Marshal(topo)
	c.Assert(err, IsNil)
	err = yaml.Unmarshal(out, &topo)
	c.Assert(err, IsNil)
	c.Assert(topo.MonitoredOptions.LogDir, Equals, "test-deploy/log")
}
