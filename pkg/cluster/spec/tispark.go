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
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/meta"
)

// TiSparkMasterSpec is the topology specification for TiSpark master node
type TiSparkMasterSpec struct {
	Host      string `yaml:"host"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	Imported  bool   `yaml:"imported,omitempty"`
	Port      int    `yaml:"port" default:"7077"`
	WebPort   int    `yaml:"web_port" default:"8080"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkMasterSpec) Role() string {
	return ComponentTiSparkMaster
}

// SSH returns the host and SSH port of the instance
func (s TiSparkMasterSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiSparkMasterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiSparkMasterSpec) IsImported() bool {
	return s.Imported
}

// TiSparkSlaveSpec is the topology specification for TiSpark slave nodes
type TiSparkSlaveSpec struct {
	Host      string `yaml:"host"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	Imported  bool   `yaml:"imported,omitempty"`
	Port      int    `yaml:"port" default:"7078"`
	WebPort   int    `yaml:"web_port" default:"8081"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkSlaveSpec) Role() string {
	return ComponentTiSparkSlave
}

// SSH returns the host and SSH port of the instance
func (s TiSparkSlaveSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiSparkSlaveSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiSparkSlaveSpec) IsImported() bool {
	return s.Imported
}

// TiSparkMasterComponent represents TiSpark master component.
type TiSparkMasterComponent struct{ *Specification }

// Name implements Component interface.
func (c *TiSparkMasterComponent) Name() string {
	return ComponentTiSparkMaster
}

// Instances implements Component interface.
func (c *TiSparkMasterComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiSparkMasters))
	for _, s := range c.TiSparkMasters {
		ins = append(ins, &TiSparkMasterInstance{
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.Port,
				sshp:         s.SSHPort,
				topo:         c.Specification,

				usedPorts: []int{
					s.Port,
					s.WebPort,
				},
				usedDirs: []string{
					s.DeployDir,
				},
				statusFn: func(_ ...string) string {
					return "-"
				},
			},
		})
	}
	return ins
}

// TiSparkMasterInstance represent the TiSpark master instance
type TiSparkMasterInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *TiSparkMasterInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}
	/*
		// transfer run script
		cfg := scripts.NewGrafanaScript(clusterName, paths.Deploy)
		fp := filepath.Join(paths.Cache, fmt.Sprintf("run_grafana_%s_%d.sh", i.GetHost(), i.GetPort()))
		if err := cfg.ConfigToFile(fp); err != nil {
			return err
		}

		dst := filepath.Join(paths.Deploy, "scripts", "run_grafana.sh")
		if err := e.Transfer(fp, dst, false); err != nil {
			return err
		}

		if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
			return err
		}

		// transfer config
		fp = filepath.Join(paths.Cache, fmt.Sprintf("grafana_%s.ini", i.GetHost()))
		if err := config.NewGrafanaConfig(i.GetHost(), paths.Deploy).WithPort(uint64(i.GetPort())).ConfigToFile(fp); err != nil {
			return err
		}
		dst = filepath.Join(paths.Deploy, "conf", "grafana.ini")
		if err := e.Transfer(fp, dst, false); err != nil {
			return err
		}

		// transfer dashboard.yml
		fp = filepath.Join(paths.Cache, fmt.Sprintf("dashboard_%s.yml", i.GetHost()))
		if err := config.NewDashboardConfig(clusterName, paths.Deploy).ConfigToFile(fp); err != nil {
			return err
		}
		dst = filepath.Join(paths.Deploy, "conf", "dashboard.yml")
		if err := e.Transfer(fp, dst, false); err != nil {
			return err
		}

		// transfer datasource.yml
		if len(i.instance.topo.Monitors) == 0 {
			return errors.New("no prometheus found in topology")
		}
		fp = filepath.Join(paths.Cache, fmt.Sprintf("datasource_%s.yml", i.GetHost()))
		if err := config.NewDatasourceConfig(clusterName, i.instance.topo.Monitors[0].Host).
			WithPort(uint64(i.instance.topo.Monitors[0].Port)).
			ConfigToFile(fp); err != nil {
			return err
		}
		dst = filepath.Join(paths.Deploy, "conf", "datasource.yml")
		return e.Transfer(fp, dst, false)
	*/
	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkMasterInstance) ScaleConfig(e executor.Executor, cluster *Specification,
	clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = cluster.Merge(i.instance.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// TiSparkSlaveComponent represents TiSpark slave component.
type TiSparkSlaveComponent struct{ *Specification }

// Name implements Component interface.
func (c *TiSparkSlaveComponent) Name() string {
	return ComponentTiSparkSlave
}

// Instances implements Component interface.
func (c *TiSparkSlaveComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiSparkSlaves))
	for _, s := range c.TiSparkSlaves {
		ins = append(ins, &TiSparkSlaveInstance{
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.Port,
				sshp:         s.SSHPort,
				topo:         c.Specification,

				usedPorts: []int{
					s.Port,
					s.WebPort,
				},
				usedDirs: []string{
					s.DeployDir,
				},
				statusFn: func(_ ...string) string {
					return "-"
				},
			},
		})
	}
	return ins
}

// TiSparkSlaveInstance represent the TiSpark slave instance
type TiSparkSlaveInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *TiSparkSlaveInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}
	/*
		// transfer run script
		cfg := scripts.NewGrafanaScript(clusterName, paths.Deploy)
		fp := filepath.Join(paths.Cache, fmt.Sprintf("run_grafana_%s_%d.sh", i.GetHost(), i.GetPort()))
		if err := cfg.ConfigToFile(fp); err != nil {
			return err
		}

		dst := filepath.Join(paths.Deploy, "scripts", "run_grafana.sh")
		if err := e.Transfer(fp, dst, false); err != nil {
			return err
		}

		if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
			return err
		}

		// transfer config
		fp = filepath.Join(paths.Cache, fmt.Sprintf("grafana_%s.ini", i.GetHost()))
		if err := config.NewGrafanaConfig(i.GetHost(), paths.Deploy).WithPort(uint64(i.GetPort())).ConfigToFile(fp); err != nil {
			return err
		}
		dst = filepath.Join(paths.Deploy, "conf", "grafana.ini")
		if err := e.Transfer(fp, dst, false); err != nil {
			return err
		}

		// transfer dashboard.yml
		fp = filepath.Join(paths.Cache, fmt.Sprintf("dashboard_%s.yml", i.GetHost()))
		if err := config.NewDashboardConfig(clusterName, paths.Deploy).ConfigToFile(fp); err != nil {
			return err
		}
		dst = filepath.Join(paths.Deploy, "conf", "dashboard.yml")
		if err := e.Transfer(fp, dst, false); err != nil {
			return err
		}

		// transfer datasource.yml
		if len(i.instance.topo.Monitors) == 0 {
			return errors.New("no prometheus found in topology")
		}
		fp = filepath.Join(paths.Cache, fmt.Sprintf("datasource_%s.yml", i.GetHost()))
		if err := config.NewDatasourceConfig(clusterName, i.instance.topo.Monitors[0].Host).
			WithPort(uint64(i.instance.topo.Monitors[0].Port)).
			ConfigToFile(fp); err != nil {
			return err
		}
		dst = filepath.Join(paths.Deploy, "conf", "datasource.yml")
		return e.Transfer(fp, dst, false)
	*/
	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkSlaveInstance) ScaleConfig(e executor.Executor, cluster *Specification,
	clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = cluster.Merge(i.instance.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}
