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

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiDBSpec represents the TiDB topology specification in topology.yaml
type TiDBSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	ListenHost      string               `yaml:"listen_host,omitempty"`
	AdvertiseAddr   string               `yaml:"advertise_address,omitempty"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"4000"`
	StatusPort      int                  `yaml:"status_port" default:"10080"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
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
func (s *TiDBSpec) Role() string {
	return ComponentTiDB
}

// SSH returns the host and SSH port of the instance
func (s *TiDBSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiDBSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *TiDBSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiDBSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiDBSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// TiDBComponent represents TiDB component.
type TiDBComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiDBComponent) Name() string {
	return ComponentTiDB
}

// Role implements Component interface.
func (c *TiDBComponent) Role() string {
	return ComponentTiDB
}

// Source implements Component interface.
func (c *TiDBComponent) Source() string {
	source := c.Topology.ComponentSources.TiDB
	if source != "" {
		return source
	}
	return ComponentTiDB
}

// CalculateVersion implements the Component interface
func (c *TiDBComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.TiDB
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *TiDBComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.TiDB = version
}

// Instances implements Component interface.
func (c *TiDBComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiDBServers))
	for _, s := range c.Topology.TiDBServers {
		s := s
		ins = append(ins, &TiDBInstance{BaseInstance{
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

// TiDBInstance represent the TiDB instance
type TiDBInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *TiDBInstance) InitConfig(
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
	spec := i.InstanceSpec.(*TiDBSpec)
	version := i.CalculateVersion(clusterVersion)

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, utils.JoinHostPort(pdspec.Host, pdspec.ClientPort))
	}
	cfg := &scripts.TiDBScript{
		Port:           spec.Port,
		StatusPort:     spec.StatusPort,
		ListenHost:     i.GetListenHost(),
		AdvertiseAddr:  utils.Ternary(spec.AdvertiseAddr != "", spec.AdvertiseAddr, spec.Host).(string),
		PD:             strings.Join(pds, ","),
		SupportSecboot: tidbver.TiDBSupportSecureBoot(version),

		DeployDir: paths.Deploy,
		LogDir:    paths.Log,

		NumaNode:  spec.NumaNode,
		NumaCores: spec.NumaCores,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tidb_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_tidb.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.TiDB
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

	spec.Config, err = i.setTiProxyConfig(ctx, topo, version, spec.Config, paths)
	if err != nil {
		return err
	}

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

// setTiProxyConfig sets tiproxy session certs
func (i *TiDBInstance) setTiProxyConfig(ctx context.Context, topo *Specification, version string, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	if len(topo.TiProxyServers) == 0 || !tidbver.TiDBSupportTiproxy(version) {
		return configs, nil
	}
	if configs == nil {
		configs = make(map[string]any)
	}
	// Overwrite users' configs just like TLS configs.
	configs["security.session-token-signing-cert"] = fmt.Sprintf(
		"%s/tls/tiproxy-session.crt",
		paths.Deploy)
	configs["security.session-token-signing-key"] = fmt.Sprintf(
		"%s/tls/tiproxy-session.key",
		paths.Deploy)
	return configs, nil
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *TiDBInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	// set TLS configs
	if enableTLS {
		if configs == nil {
			configs = make(map[string]any)
		}
		configs["security.cluster-ssl-ca"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		configs["security.cluster-ssl-cert"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		configs["security.cluster-ssl-key"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	} else {
		// drainer tls config list
		tlsConfigs := []string{
			"security.cluster-ssl-ca",
			"security.cluster-ssl-cert",
			"security.cluster-ssl-key",
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

// ScaleConfig deploy temporary config on scaling
func (i *TiDBInstance) ScaleConfig(
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

func mustBeClusterTopo(topo Topology) *Specification {
	spec, ok := topo.(*Specification)
	if !ok {
		panic("must be cluster spec")
	}
	return spec
}
