// Copyright 2022 PingCAP, Inc.
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
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tidbver"
)

// TiKVCDCSpec represents the TiKVCDC topology specification in topology.yaml
type TiKVCDCSpec struct {
	Host            string               `yaml:"host"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"8600"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	DataDir         string               `yaml:"data_dir,omitempty"`
	LogDir          string               `yaml:"log_dir,omitempty"`
	Offline         bool                 `yaml:"offline,omitempty"`
	GCTTL           int64                `yaml:"gc-ttl,omitempty" validate:"gc-ttl:editable"`
	TZ              string               `yaml:"tz,omitempty" validate:"tz:editable"`
	NumaNode        string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]any       `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s *TiKVCDCSpec) Role() string {
	return ComponentTiKVCDC
}

// SSH returns the host and SSH port of the instance
func (s *TiKVCDCSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiKVCDCSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiKVCDCSpec) IsImported() bool {
	// TiDB-Ansible do not support TiKV-CDC
	return false
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiKVCDCSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// TiKVCDCComponent represents TiKV-CDC component.
type TiKVCDCComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiKVCDCComponent) Name() string {
	return ComponentTiKVCDC
}

// Role implements Component interface.
func (c *TiKVCDCComponent) Role() string {
	return ComponentTiKVCDC
}

// Instances implements Component interface.
func (c *TiKVCDCComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiKVCDCServers))
	for _, s := range c.Topology.TiKVCDCServers {
		s := s
		instance := &TiKVCDCInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			Port:         s.Port,
			SSHP:         s.SSHPort,

			Ports: []int{
				s.Port,
			},
			Dirs: []string{
				s.DeployDir,
			},
			StatusFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config, _ ...string) string {
				return statusByHost(s.Host, s.Port, "/status", timeout, tlsCfg)
			},
			UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.Host, s.Port, timeout, tlsCfg)
			},
		}, c.Topology}
		if s.DataDir != "" {
			instance.Dirs = append(instance.Dirs, s.DataDir)
		}

		ins = append(ins, instance)
	}
	return ins
}

// TiKVCDCInstance represent the TiKV-CDC instance.
type TiKVCDCInstance struct {
	BaseInstance
	topo Topology
}

// ScaleConfig deploy temporary config on scaling
func (i *TiKVCDCInstance) ScaleConfig(
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
func (i *TiKVCDCInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	if !tidbver.TiKVCDCSupportDeploy(clusterVersion) {
		return errors.New("tikv-cdc only supports cluster version v6.2.0 or later")
	}

	topo := i.topo.(*Specification)
	if err := i.BaseInstance.InitConfig(ctx, e, topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}
	enableTLS := topo.GlobalOptions.TLSEnabled
	spec := i.InstanceSpec.(*TiKVCDCSpec)
	globalConfig := topo.ServerConfigs.TiKVCDC
	instanceConfig := spec.Config

	cfg := scripts.NewTiKVCDCScript(
		i.GetHost(),
		paths.Deploy,
		paths.Log,
		paths.Data[0],
		enableTLS,
		spec.GCTTL,
		spec.TZ,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).AppendEndpoints(topo.Endpoints(deployUser)...)

	// doesn't work.
	if _, err := i.setTLSConfig(ctx, false, nil, paths); err != nil {
		return err
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tikv-cdc_%s_%d.sh", i.GetHost(), i.GetPort()))

	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tikv-cdc.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	return i.MergeServerConfig(ctx, e, globalConfig, instanceConfig, paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *TiKVCDCInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	return nil, nil
}

var _ RollingUpdateInstance = &TiKVCDCInstance{}

// GetAddr return the address of this TiKV-CDC instance
func (i *TiKVCDCInstance) GetAddr() string {
	return fmt.Sprintf("%s:%d", i.GetHost(), i.GetPort())
}

// PreRestart implements RollingUpdateInstance interface.
// All errors are ignored, to trigger hard restart.
func (i *TiKVCDCInstance) PreRestart(ctx context.Context, topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error {
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
	if len(tidbTopo.TiKVCDCServers) <= 1 {
		logger.Debugf("tikv-cdc pre-restart skipped, only one capture in the topology, addr: %s", address)
		return nil
	}

	start := time.Now()
	client := api.NewTiKVCDCOpenAPIClient(ctx, []string{address}, 5*time.Second, tlsCfg)
	captures, err := client.GetAllCaptures()
	if err != nil {
		logger.Debugf("tikv-cdc pre-restart skipped, cannot get all captures, trigger hard restart, addr: %s, elapsed: %+v", address, time.Since(start))
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
		logger.Debugf("tikv-cdc pre-restart finished, cannot found the capture, trigger hard restart, captureID: %s, addr: %s, elapsed: %+v", captureID, address, time.Since(start))
		return nil
	}

	if isOwner {
		if err := client.ResignOwner(); err != nil {
			// if resign the owner failed, no more need to drain the current capture,
			// since it's not allowed by the cdc.
			// return nil to trigger hard restart.
			logger.Debugf("tikv-cdc pre-restart finished, resign owner failed, trigger hard restart, captureID: %s, addr: %s, elapsed: %+v", captureID, address, time.Since(start))
			return nil
		}
	}

	// TODO: support drain capture to make restart smooth.

	logger.Debugf("tikv-cdc pre-restart success, captureID: %s, addr: %s, elapsed: %+v", captureID, address, time.Since(start))
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *TiKVCDCInstance) PostRestart(ctx context.Context, topo Topology, tlsCfg *tls.Config) error {
	logger, ok := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	if !ok {
		panic("logger not found")
	}

	start := time.Now()
	address := i.GetAddr()

	client := api.NewTiKVCDCOpenAPIClient(ctx, []string{address}, 5*time.Second, tlsCfg)
	err := client.IsCaptureAlive()
	if err != nil {
		logger.Debugf("tikv-cdc post-restart finished, get capture status failed, addr: %s, err: %+v, elapsed: %+v", address, err, time.Since(start))
		return nil
	}

	logger.Debugf("tikv-cdc post-restart success, addr: %s, elapsed: %+v", address, time.Since(start))
	return nil
}
