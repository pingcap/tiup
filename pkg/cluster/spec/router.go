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
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

var routerService = "router"

// RouterSpec represents the router router specification in topology.yaml
type RouterSpec struct {
	Host                string `yaml:"host"`
	ManageHost          string `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	ListenHost          string `yaml:"listen_host,omitempty"`
	AdvertiseListenAddr string `yaml:"advertise_listen_addr,omitempty"`
	SSHPort             int    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	IgnoreExporter      bool   `yaml:"ignore_exporter,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name      string         `yaml:"name,omitempty"`
	Port      int            `yaml:"port" default:"3379"`
	DeployDir string         `yaml:"deploy_dir,omitempty"`
	DataDir   string         `yaml:"data_dir,omitempty"`
	LogDir    string         `yaml:"log_dir,omitempty"`
	Source    string         `yaml:"source,omitempty" validate:"source:editable"`
	NumaNode  string         `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config    map[string]any `yaml:"config,omitempty" validate:"config:ignore"`
	Arch      string         `yaml:"arch,omitempty"`
	OS        string         `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s *RouterSpec) Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string {
	if timeout < time.Second {
		timeout = statusQueryTimeout
	}

	addr := utils.JoinHostPort(s.GetManageHost(), s.Port)
	tc := api.NewRouterClient(ctx, []string{addr}, timeout, tlsCfg)

	// check health
	err := tc.CheckHealth()
	if err != nil {
		return "Down"
	}
	res := "Up"
	return res
}

// Role returns the component role of the instance
func (s *RouterSpec) Role() string {
	return ComponentRouter
}

// SSH returns the host and SSH port of the instance
func (s *RouterSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *RouterSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *RouterSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *RouterSpec) IsImported() bool {
	return false
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *RouterSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// GetAdvertiseListenURL returns AdvertiseListenURL
func (s *RouterSpec) GetAdvertiseListenURL(enableTLS bool) string {
	if s.AdvertiseListenAddr != "" {
		return s.AdvertiseListenAddr
	}
	scheme := utils.Ternary(enableTLS, "https", "http").(string)
	return fmt.Sprintf("%s://%s", scheme, utils.JoinHostPort(s.Host, s.Port))
}

// RouterComponent represents router component.
type RouterComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *RouterComponent) Name() string {
	return ComponentRouter
}

// Role implements Component interface.
func (c *RouterComponent) Role() string {
	return ComponentRouter
}

// Source implements Component interface.
func (c *RouterComponent) Source() string {
	source := c.Topology.ComponentSources.PD
	if source != "" {
		return source
	}
	return ComponentPD
}

// CalculateVersion implements the Component interface
func (c *RouterComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.Router
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *RouterComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.Router = version
}

// Instances implements Component interface.
func (c *RouterComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.RouterServers))
	for _, s := range c.Topology.RouterServers {
		ins = append(ins, &RouterInstance{
			BaseInstance: BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				ManageHost:   s.ManageHost,
				ListenHost:   utils.Ternary(s.ListenHost != "", s.ListenHost, c.Topology.BaseTopo().GlobalOptions.ListenHost).(string),
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
			},
			topo: c.Topology,
		})
	}
	return ins
}

// RouterInstance represent the router instance
type RouterInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *RouterInstance) InitConfig(
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
	spec := i.InstanceSpec.(*RouterSpec)
	scheme := utils.Ternary(enableTLS, "https", "http").(string)
	version := i.CalculateVersion(clusterVersion)

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, pdspec.GetAdvertiseClientURL(enableTLS))
	}
	cfg := &scripts.RouterScript{
		Name:               spec.Name,
		ListenURL:          fmt.Sprintf("%s://%s", scheme, utils.JoinHostPort(i.GetListenHost(), spec.Port)),
		AdvertiseListenURL: spec.GetAdvertiseListenURL(enableTLS),
		BackendEndpoints:   strings.Join(pds, ","),
		DeployDir:          paths.Deploy,
		DataDir:            paths.Data[0],
		LogDir:             paths.Log,
		NumaNode:           spec.NumaNode,
	}
	if !tidbver.PDSupportMicroservicesWithName(version) {
		cfg.Name = ""
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_router_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_router.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.Router
	// set TLS configs
	spec.Config, err = i.setTLSConfig(ctx, enableTLS, spec.Config, paths)
	if err != nil {
		return err
	}

	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), i.ComponentSource(), version, i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *RouterInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	// set TLS configs
	if enableTLS {
		if configs == nil {
			configs = make(map[string]any)
		}
		configs["security.cacert-path"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		configs["security.cert-path"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		configs["security.key-path"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	} else {
		// drainer tls config list
		tlsConfigs := []string{
			"security.cacert-path",
			"security.cert-path",
			"security.key-path",
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

// IsPrimary checks if the instance is primary
// for router, all instances are equal for currently.
func (i *RouterInstance) IsPrimary(ctx context.Context, topo Topology, tlsCfg *tls.Config) (bool, error) {
	return false, nil
}

// ScaleConfig deploy temporary config on scaling
func (i *RouterInstance) ScaleConfig(
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

var _ RollingUpdateInstance = &RouterInstance{}

// PreRestart implements RollingUpdateInstance interface.
func (i *RouterInstance) PreRestart(ctx context.Context, topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config, updcfg *UpdateConfig) error {
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *RouterInstance) PostRestart(ctx context.Context, topo Topology, tlsCfg *tls.Config, updcfg *UpdateConfig) error {
	return nil
}
