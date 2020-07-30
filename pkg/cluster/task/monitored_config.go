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
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/template"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	system "github.com/pingcap/tiup/pkg/cluster/template/systemd"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
)

// MonitoredConfig is used to generate the monitor node configuration
type MonitoredConfig struct {
	name       string
	component  string
	host       string
	globResCtl meta.ResourceControl
	options    *spec.MonitoredOptions
	deployUser string
	paths      meta.DirPaths
}

// Execute implements the Task interface
func (m *MonitoredConfig) Execute(ctx *Context) error {
	ports := map[string]int{
		spec.ComponentNodeExporter:     m.options.NodeExporterPort,
		spec.ComponentBlackboxExporter: m.options.BlackboxExporterPort,
	}
	// Copy to remote server
	exec, found := ctx.GetExecutor(m.host)
	if !found {
		return ErrNoExecutor
	}

	if err := os.MkdirAll(m.paths.Cache, 0755); err != nil {
		return err
	}

	if err := m.syncMonitoredSystemConfig(exec, m.component, ports[m.component]); err != nil {
		return err
	}

	var cfg template.ConfigGenerator
	if m.component == spec.ComponentNodeExporter {
		if err := m.syncBlackboxConfig(exec, config.NewBlackboxConfig()); err != nil {
			return err
		}
		cfg = scripts.NewNodeExporterScript(
			m.paths.Deploy,
			m.paths.Log,
		).WithPort(uint64(m.options.NodeExporterPort)).
			WithNumaNode(m.options.NumaNode)
	} else if m.component == spec.ComponentBlackboxExporter {
		cfg = scripts.NewBlackboxExporterScript(
			m.paths.Deploy,
			m.paths.Log,
		).WithPort(uint64(m.options.BlackboxExporterPort))
	} else {
		return fmt.Errorf("unknown monitored component %s", m.component)
	}

	return m.syncMonitoredScript(exec, m.component, cfg)
}

func (m *MonitoredConfig) syncMonitoredSystemConfig(exec executor.Executor, comp string, port int) error {
	sysCfg := filepath.Join(m.paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, m.host, port))

	resource := spec.MergeResourceControl(m.globResCtl, m.options.ResourceControl)
	systemCfg := system.NewConfig(comp, m.deployUser, m.paths.Deploy).
		WithMemoryLimit(resource.MemoryLimit).
		WithCPUQuota(resource.CPUQuota).
		WithIOReadBandwidthMax(resource.IOReadBandwidthMax).
		WithIOWriteBandwidthMax(resource.IOWriteBandwidthMax)

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return err
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := exec.Transfer(sysCfg, tgt, false); err != nil {
		return err
	}
	if outp, errp, err := exec.Execute(fmt.Sprintf("mv %s /etc/systemd/system/%s-%d.service", tgt, comp, port), true); err != nil {
		if len(outp) > 0 {
			fmt.Println(string(outp))
		}
		if len(errp) > 0 {
			log.Errorf(string(errp))
		}
		return err
	}
	return nil
}

func (m *MonitoredConfig) syncMonitoredScript(exec executor.Executor, comp string, cfg template.ConfigGenerator) error {
	fp := filepath.Join(m.paths.Cache, fmt.Sprintf("run_%s_%s.sh", comp, m.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(m.paths.Deploy, "scripts", fmt.Sprintf("run_%s.sh", comp))
	if err := exec.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := exec.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	return nil
}

func (m *MonitoredConfig) syncBlackboxConfig(exec executor.Executor, cfg template.ConfigGenerator) error {
	fp := filepath.Join(m.paths.Cache, fmt.Sprintf("blackbox_%s.yaml", m.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(m.paths.Deploy, "conf", "blackbox.yml")
	return exec.Transfer(fp, dst, false)
}

// Rollback implements the Task interface
func (m *MonitoredConfig) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (m *MonitoredConfig) String() string {
	return fmt.Sprintf("MonitoredConfig: cluster=%s, user=%s, node_exporter_port=%d, blackbox_exporter_port=%d, %v",
		m.name, m.deployUser, m.options.NodeExporterPort, m.options.BlackboxExporterPort, m.paths)
}
