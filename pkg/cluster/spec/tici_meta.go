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

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiCIMetaSpec represents the TiCI-Meta topology specification in topology.yaml
type TiCIMetaSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	ListenHost      string               `yaml:"listen_host,omitempty"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	AdvertiseHost   string               `yaml:"advertise_host,omitempty"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"8500"`
	StatusPort      int                  `yaml:"status_port" default:"8501"`
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
func (s *TiCIMetaSpec) Role() string {
	return ComponentTiCIMeta
}

// SSH returns the host and SSH port of the instance
func (s *TiCIMetaSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiCIMetaSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *TiCIMetaSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiCIMetaSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiCIMetaSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// TiCIMetaComponent represents TiCI Meta Server component.
type TiCIMetaComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiCIMetaComponent) Name() string {
	return ComponentTiCIMeta
}

// Role implements Component interface.
func (c *TiCIMetaComponent) Role() string {
	return ComponentTiCIMeta
}

// Source implements Component interface.
func (c *TiCIMetaComponent) Source() string {
	source := c.Topology.ComponentSources.TiCI
	if source != "" {
		return source
	}
	return ComponentTiCI
}

// CalculateVersion implements the Component interface
func (c *TiCIMetaComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.TiCIMeta
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *TiCIMetaComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.TiCIMeta = version
}

// Instances implements Component interface.
func (c *TiCIMetaComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiCIMetaServers))
	for _, s := range c.Topology.TiCIMetaServers {
		ins = append(ins, &TiCIMetaInstance{BaseInstance{
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

// TiCIMetaInstance represent the TiCI Meta Server instance
type TiCIMetaInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *TiCIMetaInstance) InitConfig(
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

	spec := i.InstanceSpec.(*TiCIMetaSpec)

	// Generate PD endpoints for TiCI Meta
	// currently TiCI Meta only supports single PD
	pd := utils.JoinHostPort(topo.PDServers[0].Host, topo.PDServers[0].ClientPort)

	cfg := &scripts.TiCIMetaScript{
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

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tici-meta_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_tici-meta.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.TiCIMeta
	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), i.ComponentSource(), i.CalculateVersion(clusterVersion), i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// PrepareStart implements the Instance interface
func (i *TiCIMetaInstance) PrepareStart(ctx context.Context, tlsCfg *tls.Config) error {
	topo := i.topo.(*Specification)
	if len(topo.CDCServers) == 0 {
		return errors.New("TiCDC is required for TiCI, please add at least one TiCDC instance")
	}
	spec := i.InstanceSpec.(*TiCIMetaSpec)
	var (
		bucket        = "ticidefaultbucket"
		prefix        = "tici_default_prefix"
		endpoint      = "http://localhost:9000"
		accessKey     = "minioadmin"
		secretKey     = "minioadmin"
		flushInterval = "5s" // make it configurable later
	)
	if v, ok := spec.Config["s3.bucket"].(string); ok {
		bucket = v
	}
	if v, ok := spec.Config["s3.prefix"].(string); ok {
		prefix = v
	}
	if v, ok := spec.Config["s3.endpoint"].(string); ok {
		endpoint = v
	}
	if v, ok := spec.Config["s3.access_key"].(string); ok {
		accessKey = v
	}
	if v, ok := spec.Config["s3.secret_key"].(string); ok {
		secretKey = v
	}
	cdcAddr := utils.JoinHostPort(topo.CDCServers[0].Host, topo.CDCServers[0].Port)
	cdcClient := api.NewCDCOpenAPIClient(ctx, []string{cdcAddr}, 10*time.Second, nil)
	return cdcClient.CreateChangefeed(bucket, prefix, endpoint, accessKey, secretKey, flushInterval)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiCIMetaInstance) ScaleConfig(
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
