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
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// AlertmanagerSpec represents the AlertManager topology specification in topology.yaml
type AlertmanagerSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	WebPort         int                  `yaml:"web_port" default:"9093"`
	ClusterPort     int                  `yaml:"cluster_port" default:"9094"`
	ListenHost      string               `yaml:"listen_host,omitempty" validate:"listen_host:editable"`
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
func (s *AlertmanagerSpec) Role() string {
	return ComponentAlertmanager
}

// SSH returns the host and SSH port of the instance
func (s *AlertmanagerSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *AlertmanagerSpec) GetMainPort() int {
	return s.WebPort
}

// GetManageHost returns the manage host of the instance
func (s *AlertmanagerSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *AlertmanagerSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *AlertmanagerSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// AlertManagerComponent represents Alertmanager component.
type AlertManagerComponent struct{ Topology }

// Name implements Component interface.
func (c *AlertManagerComponent) Name() string {
	return ComponentAlertmanager
}

// Role implements Component interface.
func (c *AlertManagerComponent) Role() string {
	return RoleMonitor
}

// Source implements Component interface.
func (c *AlertManagerComponent) Source() string {
	return ComponentAlertmanager
}

// CalculateVersion implements the Component interface
func (c *AlertManagerComponent) CalculateVersion(_ string) string {
	// always not follow cluster version, use ""(latest) by default
	version := c.Topology.BaseTopo().AlertManagerVersion
	if version != nil {
		return *version
	}
	return ""
}

// SetVersion implements Component interface.
func (c *AlertManagerComponent) SetVersion(version string) {
	*c.Topology.BaseTopo().AlertManagerVersion = version
}

// Instances implements Component interface.
func (c *AlertManagerComponent) Instances() []Instance {
	alertmanagers := c.Topology.BaseTopo().Alertmanagers

	ins := make([]Instance, 0, len(alertmanagers))

	for _, s := range alertmanagers {
		s := s
		ins = append(ins, &AlertManagerInstance{
			BaseInstance: BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				ManageHost:   s.ManageHost,
				ListenHost:   utils.Ternary(s.ListenHost != "", s.ListenHost, c.Topology.BaseTopo().GlobalOptions.ListenHost).(string),
				Port:         s.WebPort,
				SSHP:         s.SSHPort,
				NumaNode:     s.NumaNode,
				NumaCores:    "",

				Ports: []int{
					s.WebPort,
					s.ClusterPort,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: func(_ context.Context, timeout time.Duration, _ *tls.Config, _ ...string) string {
					return statusByHost(s.GetManageHost(), s.WebPort, "/-/ready", timeout, nil)
				},
				UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
					return UptimeByHost(s.GetManageHost(), s.WebPort, timeout, tlsCfg)
				},
				Component: c,
			},
			topo: c.Topology,
		})
	}
	return ins
}

// AlertManagerInstance represent the alert manager instance
type AlertManagerInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *AlertManagerInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	gOpts := *i.topo.BaseTopo().GlobalOptions
	if err := i.BaseInstance.InitConfig(ctx, e, gOpts, deployUser, paths); err != nil {
		return err
	}

	// Transfer start script
	spec := i.InstanceSpec.(*AlertmanagerSpec)

	peers := []string{}
	for _, amspec := range i.topo.BaseTopo().Alertmanagers {
		peers = append(peers, utils.JoinHostPort(amspec.Host, amspec.ClusterPort))
	}
	cfg := &scripts.AlertManagerScript{
		WebListenAddr:  utils.JoinHostPort(i.GetListenHost(), spec.WebPort),
		WebExternalURL: fmt.Sprintf("http://%s", utils.JoinHostPort(spec.Host, spec.WebPort)),
		ClusterPeers:   peers,
		// ClusterListenAddr cannot use i.GetListenHost due to https://github.com/prometheus/alertmanager/issues/2284 and https://github.com/prometheus/alertmanager/issues/1271
		ClusterListenAddr: utils.JoinHostPort(i.GetHost(), spec.ClusterPort),

		DeployDir: paths.Deploy,
		LogDir:    paths.Log,
		DataDir:   paths.Data[0],

		NumaNode: spec.NumaNode,
	}

	// doesn't work
	if _, err := i.setTLSConfig(ctx, false, nil, paths); err != nil {
		return err
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_alertmanager_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_alertmanager.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	dst = filepath.Join(paths.Deploy, "conf", "alertmanager.yml")
	if spec.ConfigFilePath != "" {
		return i.TransferLocalConfigFile(ctx, e, spec.ConfigFilePath, dst)
	}
	configPath := filepath.Join(paths.Cache, fmt.Sprintf("alertmanager_%s.yml", i.GetHost()))
	if err := config.NewAlertManagerConfig().ConfigToFile(configPath); err != nil {
		return err
	}
	if err := i.TransferLocalConfigFile(ctx, e, configPath, dst); err != nil {
		return err
	}
	// version is not used for alertmanager
	return checkConfig(ctx, e, i.ComponentName(), i.ComponentSource(), "", i.OS(), i.Arch(), i.ComponentName()+".yml", paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *AlertManagerInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName string,
	clusterVersion string,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *AlertManagerInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	return nil, nil
}
