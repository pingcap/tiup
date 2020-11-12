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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// PumpSpec represents the Pump topology specification in topology.yaml
type PumpSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8250"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	Offline         bool                   `yaml:"offline,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s PumpSpec) Role() string {
	return ComponentPump
}

// SSH returns the host and SSH port of the instance
func (s PumpSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s PumpSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s PumpSpec) IsImported() bool {
	return s.Imported
}

// PumpComponent represents Pump component.
type PumpComponent struct{ *Specification }

// Name implements Component interface.
func (c *PumpComponent) Name() string {
	return ComponentPump
}

// Role implements Component interface.
func (c *PumpComponent) Role() string {
	return ComponentPump
}

// Instances implements Component interface.
func (c *PumpComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.PumpServers))
	for _, s := range c.PumpServers {
		s := s
		ins = append(ins, &PumpInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			Port:         s.Port,
			SSHP:         s.SSHPort,

			Ports: []int{
				s.Port,
			},
			Dirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			StatusFn: func(tlsCfg *tls.Config, _ ...string) string {
				scheme := "http"
				if tlsCfg != nil {
					scheme = "https"
				}
				url := fmt.Sprintf("%s://%s:%d/status", scheme, s.Host, s.Port)
				return statusByURL(url, tlsCfg)
			},
		}, c.Specification})
	}
	return ins
}

// PumpInstance represent the Pump instance.
type PumpInstance struct {
	BaseInstance
	topo *Specification
}

// ScaleConfig deploy temporary config on scaling
func (i *PumpInstance) ScaleConfig(
	e executor.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() {
		i.topo = s
	}()
	i.topo = mustBeClusterTopo(topo)

	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// InitConfig implements Instance interface.
func (i *PumpInstance) InitConfig(
	e executor.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	enableTLS := i.topo.GlobalOptions.TLSEnabled
	spec := i.InstanceSpec.(PumpSpec)
	nodeID := i.GetHost() + ":" + strconv.Itoa(i.GetPort())
	// keep origin node id if is imported
	if i.IsImported() {
		nodeID = ""
	}
	cfg := scripts.NewPumpScript(
		nodeID,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).AppendEndpoints(i.topo.Endpoints(deployUser)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pump_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_pump.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	globalConfig := i.topo.ServerConfigs.Pump
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

	// set TLS configs
	if enableTLS {
		if spec.Config == nil {
			spec.Config = make(map[string]interface{})
		}
		spec.Config["security.ssl-ca"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		spec.Config["security.ssl-cert"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		spec.Config["security.ssl-key"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	}

	return i.MergeServerConfig(e, globalConfig, spec.Config, paths)
}
