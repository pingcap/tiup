// Copyright 2025 PingCAP, Inc.
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
	"path/filepath"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiCIWorkerSpec represents the TiCI-Worker topology specification in topology.yaml
type TiCIWorkerSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	ListenHost      string               `yaml:"listen_host,omitempty"`
	AdvertiseHost   string               `yaml:"advertise_host,omitempty"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"8510"`
	StatusPort      int                  `yaml:"status_port" default:"8511"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	DataDir         string               `yaml:"data_dir,omitempty"`
	LogDir          string               `yaml:"log_dir,omitempty"`
	Source          string               `yaml:"source,omitempty" validate:"source:editable"`
	NumaNode        string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	NumaCores       string               `yaml:"numa_cores,omitempty" validate:"numa_cores:editable"`
	Config          map[string]any       `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s *TiCIWorkerSpec) Role() string {
	return ComponentTiCIWorker
}

// SSH returns the host and SSH port of the instance
func (s *TiCIWorkerSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiCIWorkerSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *TiCIWorkerSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiCIWorkerSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiCIWorkerSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// TiCIWorkerComponent represents TiCI Worker Server component.
type TiCIWorkerComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiCIWorkerComponent) Name() string {
	return ComponentTiCIWorker
}

// Role implements Component interface.
func (c *TiCIWorkerComponent) Role() string {
	return ComponentTiCIWorker
}

// Source implements Component interface.
func (c *TiCIWorkerComponent) Source() string {
	source := c.Topology.ComponentSources.TiCI
	if source != "" {
		return source
	}
	return ComponentTiCI
}

// CalculateVersion implements the Component interface
func (c *TiCIWorkerComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.TiCIWorker
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *TiCIWorkerComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.TiCIWorker = version
}

// Instances implements Component interface.
func (c *TiCIWorkerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiCIWorkerServers))
	for _, s := range c.Topology.TiCIWorkerServers {
		ins = append(ins, &TiCIWorkerInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			ManageHost:   s.ManageHost,
			ListenHost:   utils.Ternary(s.ListenHost != "", s.ListenHost, c.Topology.BaseTopo().GlobalOptions.ListenHost).(string),
			Port:         s.Port,
			SSHP:         s.SSHPort,
			Source:       s.Source,
			NumaNode:     s.NumaNode,
			NumaCores:    s.NumaCores,

			Ports: []int{
				s.Port,
				s.StatusPort,
			},
			Dirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			StatusFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config, _ ...string) string {
				return statusByHost(s.GetManageHost(), s.StatusPort, "/status", timeout, tlsCfg)
			},
			UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.GetManageHost(), s.StatusPort, timeout, tlsCfg)
			},
			Component: c,
		}, c.Topology})
	}
	return ins
}

// TiCIWorkerInstance represent the TiCI Worker instance
type TiCIWorkerInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *TiCIWorkerInstance) InitConfig(
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

	spec := i.InstanceSpec.(*TiCIWorkerSpec)

	// Generate PD endpoints for TiCI Worker
	// currently TiCI Worker only supports single PD
	pd := utils.JoinHostPort(topo.PDServers[0].Host, topo.PDServers[0].ClientPort)

	cfg := &scripts.TiCIWorkerScript{
		Port:          spec.Port,
		StatusPort:    spec.StatusPort,
		ListenHost:    i.GetListenHost(),
		PD:            pd,
		AdvertiseHost: utils.Ternary(spec.AdvertiseHost != "", spec.AdvertiseHost, spec.Host).(string),

		DeployDir: paths.Deploy,
		LogDir:    paths.Log,

		NumaNode:  spec.NumaNode,
		NumaCores: spec.NumaCores,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tici-worker_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_tici-worker.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.TiCIWorker
	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), i.ComponentSource(), i.CalculateVersion(clusterVersion), i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiCIWorkerInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = mustBeClusterTopo(topo)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}
