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
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// DashboardSpec represents the Dashboard topology specification in topology.yaml
type DashboardSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"12333"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	DataDir         string               `yaml:"data_dir,omitempty"`
	LogDir          string               `yaml:"log_dir,omitempty"`
	Source          string               `yaml:"source,omitempty" validate:"source:editable"`
	NumaNode        string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]any       `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s *DashboardSpec) Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string {
	if timeout < time.Second {
		timeout = statusQueryTimeout
	}

	state := statusByHost(s.GetManageHost(), s.Port, "/status", timeout, tlsCfg)

	return state
}

// Role returns the component role of the instance
func (s *DashboardSpec) Role() string {
	return ComponentDashboard
}

// SSH returns the host and SSH port of the instance
func (s *DashboardSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *DashboardSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *DashboardSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *DashboardSpec) IsImported() bool {
	// TiDB-Ansible do not support dashboard
	return false
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *DashboardSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// DashboardComponent represents Drainer component.
type DashboardComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *DashboardComponent) Name() string {
	return ComponentDashboard
}

// Role implements Component interface.
func (c *DashboardComponent) Role() string {
	return ComponentDashboard
}

// Source implements Component interface.
func (c *DashboardComponent) Source() string {
	source := c.Topology.ComponentSources.Dashboard
	if source != "" {
		return source
	}
	return ComponentDashboard
}

// CalculateVersion implements the Component interface
func (c *DashboardComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.Dashboard
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *DashboardComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.Dashboard = version
}

// Instances implements Component interface.
func (c *DashboardComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.Drainers))
	for _, s := range c.Topology.DashboardServers {
		s := s
		ins = append(ins, &DashboardInstance{BaseInstance{
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

// DashboardInstance represent the Ddashboard instance.
type DashboardInstance struct {
	BaseInstance
	topo Topology
}

// ScaleConfig deploy temporary config on scaling
func (i *DashboardInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	user string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() {
		i.topo = s
	}()
	i.topo = mustBeClusterTopo(topo)

	return i.InitConfig(ctx, e, clusterName, clusterVersion, user, paths)
}

// InitConfig implements Instance interface.
func (i *DashboardInstance) InitConfig(
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
	spec := i.InstanceSpec.(*DashboardSpec)

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, pdspec.GetAdvertiseClientURL(enableTLS))
	}
	cfg := &scripts.DashboardScript{
		// -h, --host string              listen host of the Dashboard Server
		Host:        i.GetListenHost(),
		TidbVersion: clusterVersion,
		DeployDir:   paths.Deploy,
		DataDir:     paths.Data[0],
		LogDir:      paths.Log,
		Port:        spec.Port,
		NumaNode:    spec.NumaNode,
		PD:          strings.Join(pds, ","),
		TLSEnabled:  enableTLS,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tidb-dashboard_%s_%d.sh", i.GetHost(), i.GetPort()))

	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tidb-dashboard.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.Dashboard

	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), i.ComponentSource(), i.CalculateVersion(clusterVersion), i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *DashboardInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	return nil, nil
}
