// Copyright 2026 PingCAP, Inc.
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
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiKVWorkerSpec represents the TiKV-worker topology specification in topology.yaml.
type TiKVWorkerSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"19000"`
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

// Status queries current status of the instance.
func (s *TiKVWorkerSpec) Status(_ context.Context, timeout time.Duration, tlsCfg *tls.Config, _ ...string) string {
	return statusByHost(s.GetManageHost(), s.Port, "/healthz", timeout, tlsCfg)
}

// Role returns the component role of the instance.
func (s *TiKVWorkerSpec) Role() string {
	return ComponentTiKVWorker
}

// SSH returns the host and SSH port of the instance.
func (s *TiKVWorkerSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance.
func (s *TiKVWorkerSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance.
func (s *TiKVWorkerSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available.
func (s *TiKVWorkerSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// TiKVWorkerComponent represents TiKV-worker component.
type TiKVWorkerComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiKVWorkerComponent) Name() string {
	return ComponentTiKVWorker
}

// Role implements Component interface.
func (c *TiKVWorkerComponent) Role() string {
	return ComponentTiKVWorker
}

// Source implements Component interface.
func (c *TiKVWorkerComponent) Source() string {
	if source := c.Topology.ComponentSources.TiKVWorker; source != "" {
		return source
	}
	return ComponentTiKVWorker
}

// CalculateVersion implements Component interface.
func (c *TiKVWorkerComponent) CalculateVersion(clusterVersion string) string {
	if version := c.Topology.ComponentVersions.TiKVWorker; version != "" {
		return version
	}
	return clusterVersion
}

// SetVersion implements Component interface.
func (c *TiKVWorkerComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.TiKVWorker = version
}

// Instances implements Component interface.
func (c *TiKVWorkerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiKVWorkerServers))
	for _, s := range c.Topology.TiKVWorkerServers {
		ins = append(ins, &TiKVWorkerInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			ManageHost:   s.ManageHost,
			Port:         s.Port,
			SSHP:         s.SSHPort,
			Source:       s.Source,
			NumaNode:     s.NumaNode,
			NumaCores:    s.NumaCores,
			Ports:        []int{s.Port},
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

// TiKVWorkerInstance represents the TiKV-worker instance.
type TiKVWorkerInstance struct {
	BaseInstance
	topo Topology
}

// ScaleConfig deploy temporary config on scaling.
func (i *TiKVWorkerInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	orig := i.topo
	defer func() { i.topo = orig }()
	i.topo = mustBeClusterTopo(topo)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

// InitConfig implements Instance interface.
func (i *TiKVWorkerInstance) InitConfig(
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
	spec := i.InstanceSpec.(*TiKVWorkerSpec)

	pds := make([]string, 0, len(topo.PDServers))
	for _, pdspec := range topo.PDServers {
		pds = append(pds, pdspec.GetAdvertiseClientURL(enableTLS))
	}

	cfg := &scripts.TiKVWorkerScript{
		Addr:      utils.JoinHostPort(i.GetHost(), spec.Port),
		PD:        strings.Join(pds, ","),
		DeployDir: paths.Deploy,
		LogDir:    paths.Log,
		NumaNode:  spec.NumaNode,
		NumaCores: spec.NumaCores,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tikv-worker_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tikv-worker.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.TiKVWorker

	var err error
	spec.Config, err = i.setTLSConfig(ctx, enableTLS, spec.Config, paths)
	if err != nil {
		return err
	}
	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	if len(pds) == 0 {
		return errors.New("tikv-worker requires at least one PD server")
	}

	return checkConfig(ctx, e, i.ComponentName(), i.ComponentSource(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// setTLSConfig sets TLS config for TiKV-worker.
func (i *TiKVWorkerInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	if enableTLS {
		if configs == nil {
			configs = make(map[string]any)
		}
		configs["security.ca-path"] = fmt.Sprintf("%s/tls/%s", paths.Deploy, TLSCACert)
		configs["security.cert-path"] = fmt.Sprintf("%s/tls/%s.crt", paths.Deploy, i.Role())
		configs["security.key-path"] = fmt.Sprintf("%s/tls/%s.pem", paths.Deploy, i.Role())
		return configs, nil
	}

	// delete TLS configs
	if configs == nil {
		return nil, nil
	}
	for _, key := range []string{
		"security.ca-path",
		"security.cert-path",
		"security.key-path",
	} {
		delete(configs, key)
	}
	return configs, nil
}
