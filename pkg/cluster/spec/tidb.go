// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package spec

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiDBSpec represents the TiDB topology specification in topology.yaml
type TiDBSpec struct {
	Host            string                 `yaml:"host"`
	ListenHost      string                 `yaml:"listen_host,omitempty"`
	SSHPort         int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"4000"`
	StatusPort      int                    `yaml:"status_port" default:"10080"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// statusByURL queries current status of the instance by http status api.
func statusByURL(url string) string {
	client := utils.NewHTTPClient(statusQueryTimeout, nil)

	// body doesn't have any status section needed
	body, err := client.Get(url)
	if err != nil {
		return "Down"
	}
	if body == nil {
		return "Down"
	}
	return "Up"

}

// Status queries current status of the instance
func (s TiDBSpec) Status(pdList ...string) string {
	url := fmt.Sprintf("http://%s:%d/status", s.Host, s.StatusPort)
	return statusByURL(url)
}

// Role returns the component role of the instance
func (s TiDBSpec) Role() string {
	return ComponentTiDB
}

// SSH returns the host and SSH port of the instance
func (s TiDBSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiDBSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiDBSpec) IsImported() bool {
	return s.Imported
}

// TiDBComponent represents TiDB component.
type TiDBComponent struct{ *Specification }

// Name implements Component interface.
func (c *TiDBComponent) Name() string {
	return ComponentTiDB
}

// Instances implements Component interface.
func (c *TiDBComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiDBServers))
	for _, s := range c.TiDBServers {
		s := s
		ins = append(ins, &TiDBInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			listenHost:   s.ListenHost,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.Specification,

			usedPorts: []int{
				s.Port,
				s.StatusPort,
			},
			usedDirs: []string{
				s.DeployDir,
			},
			statusFn: s.Status,
		}})
	}
	return ins
}

// TiDBInstance represent the TiDB instance
type TiDBInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *TiDBInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(TiDBSpec)
	cfg := scripts.NewTiDBScript(
		i.GetHost(),
		paths.Deploy,
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).
		WithStatusPort(spec.StatusPort).
		AppendEndpoints(i.instance.topo.Endpoints(deployUser)...).
		WithListenHost(i.GetListenHost())
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tidb_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_tidb.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	globalConfig := i.instance.topo.ServerConfigs.TiDB
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			AnsibleImportedConfigPath,
			fmt.Sprintf(
				"%s-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		globalConfig, err = mergeImported(importConfig, globalConfig)
		if err != nil {
			return err
		}
	}

	if err := i.mergeServerConfig(e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiDBInstance) ScaleConfig(e executor.Executor, topo Topology, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = mustBeClusterTopo(topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

func mustBeClusterTopo(topo Topology) *Specification {
	spec, ok := topo.(*Specification)
	if !ok {
		panic("must be cluster spec")
	}
	return spec
}
