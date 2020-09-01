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
	"path/filepath"

	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// AlertManagerSpec represents the AlertManager topology specification in topology.yaml
type AlertManagerSpec struct {
	Host            string               `yaml:"host"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	WebPort         int                  `yaml:"web_port" default:"9093"`
	ClusterPort     int                  `yaml:"cluster_port" default:"9094"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	DataDir         string               `yaml:"data_dir,omitempty"`
	LogDir          string               `yaml:"log_dir,omitempty"`
	NumaNode        string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
	ConfigFilePath  string               `yaml:"config_file,omitempty" validate:"config_file:editable"`
}

// Role returns the component role of the instance
func (s AlertManagerSpec) Role() string {
	return ComponentAlertManager
}

// SSH returns the host and SSH port of the instance
func (s AlertManagerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s AlertManagerSpec) GetMainPort() int {
	return s.WebPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s AlertManagerSpec) IsImported() bool {
	return s.Imported
}

// AlertManagerComponent represents Alertmanager component.
type AlertManagerComponent struct{ *Specification }

// Name implements Component interface.
func (c *AlertManagerComponent) Name() string {
	return ComponentAlertManager
}

// Role implements Component interface.
func (c *AlertManagerComponent) Role() string {
	return RoleMonitor
}

// Instances implements Component interface.
func (c *AlertManagerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Alertmanager))
	for _, s := range c.Alertmanager {
		ins = append(ins, &AlertManagerInstance{
			BaseInstance: BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				Port:         s.WebPort,
				SSHP:         s.SSHPort,

				Ports: []int{
					s.WebPort,
					s.ClusterPort,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: func(_ ...string) string {
					return "-"
				},
			},
			topo: c.Specification,
		})
	}
	return ins
}

// AlertManagerInstance represent the alert manager instance
type AlertManagerInstance struct {
	BaseInstance
	topo *Specification
}

// InitConfig implement Instance interface
func (i *AlertManagerInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	// Transfer start script
	spec := i.InstanceSpec.(AlertManagerSpec)
	cfg := scripts.NewAlertManagerScript(spec.Host, paths.Deploy, paths.Data[0], paths.Log).
		WithWebPort(spec.WebPort).WithClusterPort(spec.ClusterPort).WithNumaNode(spec.NumaNode).
		AppendEndpoints(AlertManagerEndpoints(i.topo.Alertmanager, deployUser))

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_alertmanager_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_alertmanager.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	dst = filepath.Join(paths.Deploy, "conf", "alertmanager.yml")
	if spec.ConfigFilePath != "" {
		return i.TransferLocalConfigFile(e, spec.ConfigFilePath, dst)
	}
	configPath := filepath.Join(paths.Cache, fmt.Sprintf("alertmanager_%s.yml", i.GetHost()))
	if err := config.NewAlertManagerConfig().ConfigToFile(configPath); err != nil {
		return err
	}
	return i.TransferLocalConfigFile(e, configPath, dst)
}

// ScaleConfig deploy temporary config on scaling
func (i *AlertManagerInstance) ScaleConfig(e executor.Executor, topo Topology,
	clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = mustBeClusterTopo(topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}
