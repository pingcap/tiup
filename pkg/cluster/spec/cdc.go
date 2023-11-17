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

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

// CDCSpec represents the CDC topology specification in topology.yaml
type CDCSpec struct {
	Host            string               `yaml:"host"`
	ManageHost      string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"8300"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	DataDir         string               `yaml:"data_dir,omitempty"`
	LogDir          string               `yaml:"log_dir,omitempty"`
	Offline         bool                 `yaml:"offline,omitempty"`
	GCTTL           int64                `yaml:"gc-ttl,omitempty" validate:"gc-ttl:editable"`
	TZ              string               `yaml:"tz,omitempty" validate:"tz:editable"`
	TiCDCClusterID  string               `yaml:"ticdc_cluster_id"`
	Source          string               `yaml:"source,omitempty" validate:"source:editable"`
	NumaNode        string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]any       `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s *CDCSpec) Role() string {
	return ComponentCDC
}

// SSH returns the host and SSH port of the instance
func (s *CDCSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *CDCSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *CDCSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *CDCSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *CDCSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// CDCComponent represents CDC component.
type CDCComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *CDCComponent) Name() string {
	return ComponentCDC
}

// Role implements Component interface.
func (c *CDCComponent) Role() string {
	return ComponentCDC
}

// Source implements Component interface.
func (c *CDCComponent) Source() string {
	source := c.Topology.ComponentSources.CDC
	if source != "" {
		return source
	}
	return ComponentCDC
}

// CalculateVersion implements the Component interface
func (c *CDCComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.CDC
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *CDCComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.CDC = version
}

// Instances implements Component interface.
func (c *CDCComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.CDCServers))
	for _, s := range c.Topology.CDCServers {
		s := s
		instance := &CDCInstance{BaseInstance{
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
			},
			StatusFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config, _ ...string) string {
				return statusByHost(s.GetManageHost(), s.Port, "/status", timeout, tlsCfg)
			},
			UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.GetManageHost(), s.Port, timeout, tlsCfg)
			},
			Component: c,
		}, c.Topology}
		if s.DataDir != "" {
			instance.Dirs = append(instance.Dirs, s.DataDir)
		}

		ins = append(ins, instance)
	}
	return ins
}

// CDCInstance represent the CDC instance.
type CDCInstance struct {
	BaseInstance
	topo Topology
}

// ScaleConfig deploy temporary config on scaling
func (i *CDCInstance) ScaleConfig(
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
func (i *CDCInstance) InitConfig(
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
	spec := i.InstanceSpec.(*CDCSpec)
	globalConfig := topo.ServerConfigs.CDC
	instanceConfig := spec.Config
	version := i.CalculateVersion(clusterVersion)

	if !tidbver.TiCDCSupportConfigFile(version) {
		if len(globalConfig)+len(instanceConfig) > 0 {
			return errors.New("server_config is only supported with TiCDC version v4.0.13 or later")
		}
	}

	if !tidbver.TiCDCSupportClusterID(version) && spec.TiCDCClusterID != "" {
		return errors.New("ticdc_cluster_id is only supported with TiCDC version v6.2.0 or later")
	}

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, pdspec.GetAdvertiseClientURL(enableTLS))
	}
	cfg := &scripts.CDCScript{
		Addr:              utils.JoinHostPort(i.GetListenHost(), spec.Port),
		AdvertiseAddr:     utils.JoinHostPort(i.GetHost(), i.GetPort()),
		PD:                strings.Join(pds, ","),
		GCTTL:             spec.GCTTL,
		TZ:                spec.TZ,
		ClusterID:         spec.TiCDCClusterID,
		DataDirEnabled:    tidbver.TiCDCSupportDataDir(version),
		ConfigFileEnabled: tidbver.TiCDCSupportConfigFile(version),
		TLSEnabled:        enableTLS,

		DeployDir: paths.Deploy,
		LogDir:    paths.Log,
		DataDir:   utils.Ternary(tidbver.TiCDCSupportSortOrDataDir(version), spec.DataDir, "").(string),

		NumaNode: spec.NumaNode,
	}

	// doesn't work
	if _, err := i.setTLSConfig(ctx, false, nil, paths); err != nil {
		return err
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_cdc_%s_%d.sh", i.GetHost(), i.GetPort()))

	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_cdc.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	return i.MergeServerConfig(ctx, e, globalConfig, instanceConfig, paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *CDCInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	return nil, nil
}

var _ RollingUpdateInstance = &CDCInstance{}

// GetAddr return the address of this TiCDC instance
func (i *CDCInstance) GetAddr() string {
	return utils.JoinHostPort(i.GetHost(), i.GetPort())
}

// PreRestart implements RollingUpdateInstance interface.
// All errors are ignored, to trigger hard restart.
func (i *CDCInstance) PreRestart(ctx context.Context, topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("should be type of tidb topology")
	}

	logger, ok := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	if !ok {
		panic("logger not found")
	}

	address := i.GetAddr()
	// cdc rolling upgrade strategy only works if there are more than 2 captures
	if len(tidbTopo.CDCServers) <= 1 {
		logger.Debugf("cdc pre-restart skipped, only one capture in the topology, addr: %s", address)
		return nil
	}

	start := time.Now()
	client := api.NewCDCOpenAPIClient(ctx, topo.(*Specification).GetCDCListWithManageHost(), 5*time.Second, tlsCfg)
	if err := client.Healthy(); err != nil {
		logger.Debugf("cdc pre-restart skipped, the cluster unhealthy, trigger hard restart, "+
			"addr: %s, err: %+v, elapsed: %+v", address, err, time.Since(start))
		return nil
	}

	captures, err := client.GetAllCaptures()
	if err != nil {
		logger.Debugf("cdc pre-restart skipped, cannot get all captures, trigger hard restart, "+
			"addr: %s, err: %+v, elapsed: %+v", address, err, time.Since(start))
		return nil
	}

	var (
		captureID string
		found     bool
		isOwner   bool
	)
	for _, capture := range captures {
		if address == capture.AdvertiseAddr {
			found = true
			captureID = capture.ID
			isOwner = capture.IsOwner
			break
		}
	}

	// this may happen if the capture crashed right away.
	if !found {
		logger.Debugf("cdc pre-restart finished, cannot found the capture, trigger hard restart, "+
			"addr: %s, elapsed: %+v", address, time.Since(start))
		return nil
	}

	if isOwner {
		if err := client.ResignOwner(address); err != nil {
			// if resign the owner failed, no more need to drain the current capture,
			// since it's not allowed by the cdc.
			// return nil to trigger hard restart.
			logger.Debugf("cdc pre-restart finished, resign owner failed, trigger hard restart, "+
				"addr: %s, captureID: %s, err: %+v, elapsed: %+v", address, captureID, err, time.Since(start))
			return nil
		}
	}

	if err := client.DrainCapture(address, captureID, apiTimeoutSeconds); err != nil {
		logger.Debugf("cdc pre-restart finished, drain the capture failed, "+
			"addr: %s, captureID: %s, err: %+v, elapsed: %+v", address, captureID, err, time.Since(start))
		// if we drain any one capture failed, no need to drain other captures, just trigger hard restart
		return nil
	}

	logger.Debugf("cdc pre-restart success, addr: %s, captureID: %s, elapsed: %+v", address, captureID, time.Since(start))
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *CDCInstance) PostRestart(ctx context.Context, topo Topology, tlsCfg *tls.Config) error {
	logger, ok := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	if !ok {
		panic("logger not found")
	}

	start := time.Now()
	address := i.GetAddr()

	client := api.NewCDCOpenAPIClient(ctx, []string{utils.JoinHostPort(i.GetManageHost(), i.GetPort())}, 5*time.Second, tlsCfg)
	err := client.IsCaptureAlive()
	if err != nil {
		logger.Debugf("cdc post-restart finished, get capture status failed, addr: %s, err: %+v, elapsed: %+v", address, err, time.Since(start))
		return nil
	}

	logger.Debugf("cdc post-restart success, addr: %s, elapsed: %+v", address, time.Since(start))
	return nil
}
