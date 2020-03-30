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
	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/template"
	"github.com/pingcap-incubator/tiops/pkg/template/config"
	"github.com/pingcap-incubator/tiops/pkg/template/scripts"
	system "github.com/pingcap-incubator/tiops/pkg/template/systemd"
)

// MonitoredConfig is used to generate the monitor node configuration
type MonitoredConfig struct {
	name       string
	component  string
	host       string
	options    meta.MonitoredOptions
	deployUser string
	deployDir  string
}

// Execute implements the Task interface
func (m *MonitoredConfig) Execute(ctx *Context) error {
	ports := map[string]int{
		meta.ComponentNodeExporter:     m.options.NodeExporterPort,
		meta.ComponentBlackboxExporter: m.options.BlackboxExporterPort,
	}
	// Copy to remote server
	exec, found := ctx.GetExecutor(m.host)
	if !found {
		return ErrNoExecutor
	}
	cacheDir := meta.ClusterPath(m.name, "config")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	if err := m.syncMonitoredSystemConfig(exec, cacheDir, m.component, ports[m.component]); err != nil {
		return err
	}

	var cfg template.ConfigGenerator
	if m.component == meta.ComponentNodeExporter {
		if err := m.syncBlackboxConfig(exec, cacheDir, config.NewBlackboxConfig()); err != nil {
			return err
		}
		cfg = scripts.NewNodeExporterScript(m.deployDir).WithPort(uint64(m.options.NodeExporterPort))
	} else if m.component == meta.ComponentBlackboxExporter {
		cfg = scripts.NewBlackboxExporterScript(m.deployDir).WithPort(uint64(m.options.BlackboxExporterPort))
	} else {
		return fmt.Errorf("unknown monitored component %s", m.component)
	}
	if err := m.syncMonitoredScript(exec, cacheDir, m.component, cfg); err != nil {
		return err
	}
	return nil
}

func (m *MonitoredConfig) syncMonitoredSystemConfig(exec executor.TiOpsExecutor, cacheDir, comp string, port int) error {
	sysCfg := filepath.Join(cacheDir, fmt.Sprintf("%s-%d.service", comp, port))
	systemCfg := system.NewConfig(comp, m.deployUser, m.deployDir)
	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return err
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := exec.Transfer(sysCfg, tgt); err != nil {
		return err
	}
	if outp, errp, err := exec.Execute(fmt.Sprintf("mv %s /etc/systemd/system/%s-%d.service", tgt, comp, port), true); err != nil {
		if len(outp) > 0 {
			log.Output(string(outp))
		}
		if len(errp) > 0 {
			log.Errorf(string(errp))
		}
		return err
	}
	return nil
}

func (m *MonitoredConfig) syncMonitoredScript(exec executor.TiOpsExecutor, cacheDir, comp string, cfg template.ConfigGenerator) error {
	fp := filepath.Join(cacheDir, fmt.Sprintf("run_%s_%s.sh", comp, m.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(m.deployDir, "scripts", fmt.Sprintf("run_%s.sh", comp))
	if err := exec.Transfer(fp, dst); err != nil {
		return err
	}
	if _, _, err := exec.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	return nil
}

func (m *MonitoredConfig) syncBlackboxConfig(exec executor.TiOpsExecutor, cacheDir string, cfg template.ConfigGenerator) error {
	fp := filepath.Join(cacheDir, fmt.Sprintf("blackbox_%s.yaml", m.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(m.deployDir, "conf", "blackbox.yml")
	if err := exec.Transfer(fp, dst); err != nil {
		return err
	}
	return nil
}

// Rollback implements the Task interface
func (m *MonitoredConfig) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (m *MonitoredConfig) String() string {
	return fmt.Sprintf("MonitoredConfig: cluster=%s, user=%s, dir=%s, node_exporter_port=%d, blackbox_exporter_port=%d",
		m.name, m.deployUser, m.deployDir, m.options.NodeExporterPort, m.options.BlackboxExporterPort)
}
