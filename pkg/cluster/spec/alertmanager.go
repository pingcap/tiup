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
)

// AlertmanagerSpec represents the AlertManager topology specification in topology.yaml
type AlertmanagerSpec struct {
	Host            string               `yaml:"host"`
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
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *AlertmanagerSpec) GetMainPort() int {
	return s.WebPort
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
				Port:         s.WebPort,
				SSHP:         s.SSHPort,

				Ports: []int{
					s.WebPort,
					s.ClusterPort,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: func(_ context.Context, _ *tls.Config, _ ...string) string {
					return statusByHost(s.Host, s.WebPort, "/-/ready", nil)
				},
				UptimeFn: func(_ context.Context, tlsCfg *tls.Config) time.Duration {
					return UptimeByHost(s.Host, s.WebPort, tlsCfg)
				},
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

	alertmanagers := i.topo.BaseTopo().Alertmanagers

	enableTLS := gOpts.TLSEnabled
	// Transfer start script
	spec := i.InstanceSpec.(*AlertmanagerSpec)
	cfg := scripts.NewAlertManagerScript(spec.Host, spec.ListenHost, paths.Deploy, paths.Data[0], paths.Log, enableTLS).
		WithWebPort(spec.WebPort).WithClusterPort(spec.ClusterPort).WithNumaNode(spec.NumaNode).
		AppendEndpoints(AlertManagerEndpoints(alertmanagers, deployUser, enableTLS))

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
	return checkConfig(ctx, e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".yml", paths, nil)
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
func (i *AlertManagerInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]interface{}, paths meta.DirPaths) (map[string]interface{}, error) {
	return nil, nil
}
