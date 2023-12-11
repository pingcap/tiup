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
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// PumpSpec represents the Pump topology specification in topology.yaml
type PumpSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"8250"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	DataDir         string               `yaml:"data_dir,omitempty"`
	LogDir          string               `yaml:"log_dir,omitempty"`
	Offline         bool                 `yaml:"offline,omitempty"`
	Source          string               `yaml:"source,omitempty" validate:"source:editable"`
	NumaNode        string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]any       `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s *PumpSpec) Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string {
	if timeout < time.Second {
		timeout = statusQueryTimeout
	}

	state := statusByHost(s.GetManageHost(), s.Port, "/status", timeout, tlsCfg)

	if s.Offline {
		binlogClient, err := api.NewBinlogClient(pdList, timeout, tlsCfg)
		if err != nil {
			return state
		}
		id := utils.JoinHostPort(s.Host, s.Port)
		tombstone, _ := binlogClient.IsPumpTombstone(ctx, id)

		if tombstone {
			state = "Tombstone"
		} else {
			state = "Pending Offline"
		}
	}
	return state
}

// Role returns the component role of the instance
func (s *PumpSpec) Role() string {
	return ComponentPump
}

// SSH returns the host and SSH port of the instance
func (s *PumpSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *PumpSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *PumpSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *PumpSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *PumpSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// PumpComponent represents Pump component.
type PumpComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *PumpComponent) Name() string {
	return ComponentPump
}

// Role implements Component interface.
func (c *PumpComponent) Role() string {
	return ComponentPump
}

// Source implements Component interface.
func (c *PumpComponent) Source() string {
	source := c.Topology.ComponentSources.Pump
	if source != "" {
		return source
	}
	return ComponentPump
}

// CalculateVersion implements the Component interface
func (c *PumpComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.Pump
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *PumpComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.Pump = version
}

// Instances implements Component interface.
func (c *PumpComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.PumpServers))
	for _, s := range c.Topology.PumpServers {
		s := s
		ins = append(ins, &PumpInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			ManageHost:   s.ManageHost,
			ListenHost:   c.Topology.BaseTopo().GlobalOptions.ListenHost,
			Port:         s.Port,
			SSHP:         s.SSHPort,
			Source:       s.Source,
			NumaNode:     s.NumaNode,
			NumaCores:    "",

			Ports: []int{
				s.Port,
			},
			Dirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			StatusFn: s.Status,
			UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.GetManageHost(), s.Port, timeout, tlsCfg)
			},
			Component: c,
		}, c.Topology})
	}
	return ins
}

// PumpInstance represent the Pump instance.
type PumpInstance struct {
	BaseInstance
	topo Topology
}

// ScaleConfig deploy temporary config on scaling
func (i *PumpInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
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

	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

// InitConfig implements Instance interface.
func (i *PumpInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	topo := i.topo.(*Specification)
	if err := i.BaseInstance.InitConfig(ctx, e, topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	enableTLS := topo.GlobalOptions.TLSEnabled
	spec := i.InstanceSpec.(*PumpSpec)
	nodeID := i.ID()
	// keep origin node id if is imported
	if i.IsImported() {
		nodeID = ""
	}

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, pdspec.GetAdvertiseClientURL(enableTLS))
	}
	cfg := &scripts.PumpScript{
		NodeID:        nodeID,
		Addr:          utils.JoinHostPort(i.GetListenHost(), spec.Port),
		AdvertiseAddr: utils.JoinHostPort(spec.Host, spec.Port),
		PD:            strings.Join(pds, ","),

		DeployDir: paths.Deploy,
		DataDir:   paths.Data[0],
		LogDir:    paths.Log,

		NumaNode: spec.NumaNode,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pump_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_pump.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.Pump
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
		importConfig, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		globalConfig, err = mergeImported(importConfig, globalConfig)
		if err != nil {
			return err
		}
	}

	// set TLS configs
	spec.Config, err = i.setTLSConfig(ctx, enableTLS, spec.Config, paths)
	if err != nil {
		return err
	}

	return i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *PumpInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	// set TLS configs
	if enableTLS {
		if configs == nil {
			configs = make(map[string]any)
		}
		configs["security.ssl-ca"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		configs["security.ssl-cert"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		configs["security.ssl-key"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	} else {
		// drainer tls config list
		tlsConfigs := []string{
			"security.ssl-ca",
			"security.ssl-cert",
			"security.ssl-key",
		}
		// delete TLS configs
		if configs != nil {
			for _, config := range tlsConfigs {
				delete(configs, config)
			}
		}
	}

	return configs, nil
}
