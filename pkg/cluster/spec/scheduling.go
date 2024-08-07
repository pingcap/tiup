// Copyright 2024 PingCAP, Inc.
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

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

var schedulingService = "scheduling"

// SchedulingSpec represents the scheduling topology specification in topology.yaml
type SchedulingSpec struct {
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
func (s *SchedulingSpec) Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string {
	if timeout < time.Second {
		timeout = statusQueryTimeout
	}

	addr := utils.JoinHostPort(s.GetManageHost(), s.Port)
	tc := api.NewSchedulingClient(ctx, []string{addr}, timeout, tlsCfg)
	pc := api.NewPDClient(ctx, pdList, timeout, tlsCfg)

	// check health
	err := tc.CheckHealth()
	if err != nil {
		return "Down"
	}

	primary, err := pc.GetServicePrimary(schedulingService)
	if err != nil {
		return "ERR"
	}
	res := "Up"
	enableTLS := false
	if tlsCfg != nil {
		enableTLS = true
	}
	if s.GetAdvertiseListenURL(enableTLS) == primary {
		res += "|P"
	}

	return res
}

// Role returns the component role of the instance
func (s *SchedulingSpec) Role() string {
	return ComponentScheduling
}

// SSH returns the host and SSH port of the instance
func (s *SchedulingSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *SchedulingSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *SchedulingSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *SchedulingSpec) IsImported() bool {
	return false
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *SchedulingSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// GetAdvertiseListenURL returns AdvertiseListenURL
func (s *SchedulingSpec) GetAdvertiseListenURL(enableTLS bool) string {
	if s.AdvertiseListenAddr != "" {
		return s.AdvertiseListenAddr
	}
	scheme := utils.Ternary(enableTLS, "https", "http").(string)
	return fmt.Sprintf("%s://%s", scheme, utils.JoinHostPort(s.Host, s.Port))
}

// SchedulingComponent represents scheduling component.
type SchedulingComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *SchedulingComponent) Name() string {
	return ComponentScheduling
}

// Role implements Component interface.
func (c *SchedulingComponent) Role() string {
	return ComponentScheduling
}

// Source implements Component interface.
func (c *SchedulingComponent) Source() string {
	source := c.Topology.ComponentSources.PD
	if source != "" {
		return source
	}
	return ComponentPD
}

// CalculateVersion implements the Component interface
func (c *SchedulingComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.Scheduling
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *SchedulingComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.Scheduling = version
}

// Instances implements Component interface.
func (c *SchedulingComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.SchedulingServers))
	for _, s := range c.Topology.SchedulingServers {
		s := s
		ins = append(ins, &SchedulingInstance{
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

// SchedulingInstance represent the scheduling instance
type SchedulingInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *SchedulingInstance) InitConfig(
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
	spec := i.InstanceSpec.(*SchedulingSpec)
	scheme := utils.Ternary(enableTLS, "https", "http").(string)
	version := i.CalculateVersion(clusterVersion)

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, pdspec.GetAdvertiseClientURL(enableTLS))
	}
	cfg := &scripts.SchedulingScript{
		Name:               spec.Name,
		ListenURL:          fmt.Sprintf("%s://%s", scheme, utils.JoinHostPort(i.GetListenHost(), spec.Port)),
		AdvertiseListenURL: spec.GetAdvertiseListenURL(enableTLS),
		BackendEndpoints:   strings.Join(pds, ","),
		DeployDir:          paths.Deploy,
		DataDir:            paths.Data[0],
		LogDir:             paths.Log,
		NumaNode:           spec.NumaNode,
	}
	if !tidbver.PDSupportMicroServicesWithName(version) {
		cfg.Name = ""
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_scheduling_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_scheduling.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.Scheduling
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
func (i *SchedulingInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
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
func (i *SchedulingInstance) IsPrimary(ctx context.Context, topo Topology, tlsCfg *tls.Config) (bool, error) {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("topo should be type of tidb topology")
	}
	pdClient := api.NewPDClient(ctx, tidbTopo.GetPDListWithManageHost(), time.Second*5, tlsCfg)
	primary, err := pdClient.GetServicePrimary(schedulingService)
	if err != nil {
		return false, errors.Annotatef(err, "failed to get Scheduling primary %s", i.GetHost())
	}

	spec := i.InstanceSpec.(*SchedulingSpec)
	enableTLS := false
	if tlsCfg != nil {
		enableTLS = true
	}

	return primary == spec.GetAdvertiseListenURL(enableTLS), nil
}

// ScaleConfig deploy temporary config on scaling
func (i *SchedulingInstance) ScaleConfig(
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

var _ RollingUpdateInstance = &SchedulingInstance{}

// PreRestart implements RollingUpdateInstance interface.
func (i *SchedulingInstance) PreRestart(ctx context.Context, topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error {
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *SchedulingInstance) PostRestart(ctx context.Context, topo Topology, tlsCfg *tls.Config) error {
	return nil
}
