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

package task

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/template"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	system "github.com/pingcap/tiup/pkg/cluster/template/systemd"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

// MonitoredConfig is used to generate the monitor node configuration
type MonitoredConfig struct {
	name        string
	component   string
	host        string
	globResCtl  meta.ResourceControl
	options     *spec.MonitoredOptions
	deployUser  string
	tlsEnabled  bool
	paths       meta.DirPaths
	systemdMode spec.SystemdMode
}

// Execute implements the Task interface
func (m *MonitoredConfig) Execute(ctx context.Context) error {
	ports := map[string]int{
		spec.ComponentNodeExporter:     m.options.NodeExporterPort,
		spec.ComponentBlackboxExporter: m.options.BlackboxExporterPort,
	}
	// Copy to remote server
	exec, found := ctxt.GetInner(ctx).GetExecutor(m.host)
	if !found {
		return ErrNoExecutor
	}

	if err := utils.MkdirAll(m.paths.Cache, 0755); err != nil {
		return err
	}

	if err := m.syncMonitoredSystemConfig(ctx, exec, m.component, ports[m.component], m.systemdMode); err != nil {
		return err
	}

	var cfg template.ConfigGenerator
	switch m.component {
	case spec.ComponentNodeExporter:
		if err := m.syncBlackboxConfig(ctx, exec, config.NewBlackboxConfig(m.paths.Deploy, m.tlsEnabled)); err != nil {
			return err
		}
		cfg = scripts.
			NewNodeExporterScript(m.paths.Deploy, m.paths.Log).
			WithPort(uint64(m.options.NodeExporterPort)).
			WithNumaNode(m.options.NumaNode)
	case spec.ComponentBlackboxExporter:
		cfg = scripts.
			NewBlackboxExporterScript(m.paths.Deploy, m.paths.Log).
			WithPort(uint64(m.options.BlackboxExporterPort))
	default:
		return fmt.Errorf("unknown monitored component %s", m.component)
	}

	return m.syncMonitoredScript(ctx, exec, m.component, cfg)
}

func (m *MonitoredConfig) syncMonitoredSystemConfig(ctx context.Context, exec ctxt.Executor, comp string, port int, systemdMode spec.SystemdMode) (err error) {
	sysCfg := filepath.Join(m.paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, m.host, port))

	// insert checkpoint
	point := checkpoint.Acquire(ctx, spec.CopyConfigFile, map[string]any{"config-file": sysCfg})
	defer func() {
		point.Release(err, zap.String("config-file", sysCfg))
	}()
	if point.Hit() != nil {
		return nil
	}

	if len(systemdMode) == 0 {
		systemdMode = spec.SystemMode
	}

	resource := spec.MergeResourceControl(m.globResCtl, m.options.ResourceControl)
	systemCfg := system.NewConfig(comp, m.deployUser, m.paths.Deploy).
		WithMemoryLimit(resource.MemoryLimit).
		WithCPUQuota(resource.CPUQuota).
		WithIOReadBandwidthMax(resource.IOReadBandwidthMax).
		WithIOWriteBandwidthMax(resource.IOWriteBandwidthMax).
		WithSystemdMode(string(systemdMode))

	// blackbox_exporter needs cap_net_raw to send ICMP ping packets
	if comp == spec.ComponentBlackboxExporter {
		systemCfg.GrantCapNetRaw = true
	}

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return err
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := exec.Transfer(ctx, sysCfg, tgt, false, 0, false); err != nil {
		return err
	}
	systemdDir := "/etc/systemd/system/"
	sudo := true
	if systemdMode == spec.UserMode {
		systemdDir = "~/.config/systemd/user/"
		sudo = false
	}
	if outp, errp, err := exec.Execute(ctx, fmt.Sprintf("mv %s %s%s-%d.service", tgt, systemdDir, comp, port), sudo); err != nil {
		if len(outp) > 0 {
			fmt.Println(string(outp))
		}
		if len(errp) > 0 {
			ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger).
				Errorf(string(errp))
		}
		return err
	}
	return nil
}

func (m *MonitoredConfig) syncMonitoredScript(ctx context.Context, exec ctxt.Executor, comp string, cfg template.ConfigGenerator) error {
	fp := filepath.Join(m.paths.Cache, fmt.Sprintf("run_%s_%s.sh", comp, m.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(m.paths.Deploy, "scripts", fmt.Sprintf("run_%s.sh", comp))
	if err := exec.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	if _, _, err := exec.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	return nil
}

func (m *MonitoredConfig) syncBlackboxConfig(ctx context.Context, exec ctxt.Executor, cfg template.ConfigGenerator) error {
	fp := filepath.Join(m.paths.Cache, fmt.Sprintf("blackbox_%s.yaml", m.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(m.paths.Deploy, "conf", "blackbox.yml")
	return exec.Transfer(ctx, fp, dst, false, 0, false)
}

// Rollback implements the Task interface
func (m *MonitoredConfig) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (m *MonitoredConfig) String() string {
	return fmt.Sprintf("MonitoredConfig: cluster=%s, user=%s, node_exporter_port=%d, blackbox_exporter_port=%d, %v",
		m.name, m.deployUser, m.options.NodeExporterPort, m.options.BlackboxExporterPort, m.paths)
}
