// Copyright 2025 PingCAP, Inc.
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

package instance

import (
	"context"
	"fmt"
	"path/filepath"
	"syscall"

	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiCIRole defines the role of TiCI instance
type TiCIRole int

const (
	// TiCIRoleMeta represents the MetaServer role
	TiCIRoleMeta TiCIRole = iota // MetaServer
	// TiCIRoleWorker represents the WorkerNode role
	TiCIRoleWorker // WorkerNode
)

// TiCIInstance represents a TiCI service instance (either MetaServer or WorkerNode)
type TiCIInstance struct {
	instance

	// TiCI specific fields
	pds  []*PDInstance
	role TiCIRole // Instance role (meta or worker)

	// Process - only one process per instance
	process *process
}

var _ Instance = &TiCIInstance{}

// NewTiCIMetaInstance creates a TiCI MetaServer instance
func NewTiCIMetaInstance(shOpt SharedOptions, baseDir, host string, id int, pds []*PDInstance,
	ticiBinaryDir, configDir string) *TiCIInstance {
	return NewTiCIInstanceWithRole(shOpt, baseDir, host, id, pds, ticiBinaryDir, configDir, TiCIRoleMeta)
}

// NewTiCIWorkerInstance creates a TiCI WorkerNode instance
func NewTiCIWorkerInstance(shOpt SharedOptions, baseDir, host string, id int, pds []*PDInstance,
	ticiBinaryDir, configDir string) *TiCIInstance {
	return NewTiCIInstanceWithRole(shOpt, baseDir, host, id, pds, ticiBinaryDir, configDir, TiCIRoleWorker)
}

// NewTiCIInstanceWithRole creates a TiCI instance with specified role
func NewTiCIInstanceWithRole(shOpt SharedOptions, baseDir, host string, id int, pds []*PDInstance,
	ticiBinaryDir, configDir string, role TiCIRole) *TiCIInstance {
	var componentSuffix string
	var defaultPort, defaultStatusPort int
	var configPath, binPath string

	switch role {
	case TiCIRoleMeta:
		// MetaServer default port
		componentSuffix = "meta"
		defaultPort = 8500
		defaultStatusPort = 8501
		if configDir != "" {
			configPath = filepath.Join(configDir, "test-meta.toml")
		} else {
			configPath = filepath.Join(ticiBinaryDir, "../../ci", "test-meta.toml")
		}
		binPath = filepath.Join(ticiBinaryDir, "meta_service_server")
	case TiCIRoleWorker:
		// WorkerNode default port
		componentSuffix = "worker"
		defaultPort = 8510
		defaultStatusPort = 8511
		if configDir != "" {
			configPath = filepath.Join(configDir, "test-worker.toml")
		} else {
			configPath = filepath.Join(ticiBinaryDir, "../../ci", "test-worker.toml")
		}
		binPath = filepath.Join(ticiBinaryDir, "worker_node_server")
	default:
		panic("invalid TiCI role")
	}

	dir := filepath.Join(baseDir, fmt.Sprintf("tici-%s-%d", componentSuffix, id))

	tici := &TiCIInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, defaultPort, shOpt.PortOffset),
			StatusPort: utils.MustGetFreePort(host, defaultStatusPort, shOpt.PortOffset),
			ConfigPath: configPath,
		},
		pds:  pds,
		role: role,
	}

	return tici
}

// Start implements Instance interface - starts the appropriate process
func (t *TiCIInstance) Start(ctx context.Context) error {
	if t.process != nil {
		return fmt.Errorf("TiCI instance already started")
	}

	if err := utils.MkdirAll(t.Dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", t.Dir, err)
	}

	return t.startInstance(ctx)
}

func (t *TiCIInstance) startInstance(ctx context.Context) error {
	args := []string{}

	// Set the config path
	args = append(args, fmt.Sprintf("--config=%s", t.ConfigPath))

	t.process = &process{cmd: PrepareCommand(ctx, t.BinPath, args, nil, t.Dir)}

	// Set up logging
	logIfErr(t.process.SetOutputFile(t.LogFile()))

	return t.process.Start()
}

// Wait implements Instance interface
func (t *TiCIInstance) Wait() error {
	if t.process != nil {
		return t.process.Wait()
	}
	return nil
}

// Pid implements Instance interface
func (t *TiCIInstance) Pid() int {
	if t.process != nil && t.process.Cmd() != nil && t.process.Cmd().Process != nil {
		return t.process.Cmd().Process.Pid
	}
	return 0
}

// Uptime implements Instance interface
func (t *TiCIInstance) Uptime() string {
	if t.process != nil {
		return t.process.Uptime()
	}
	return "N/A"
}

// Component implements Instance interface
func (t *TiCIInstance) Component() string {
	switch t.role {
	case TiCIRoleMeta:
		return "tici-meta"
	case TiCIRoleWorker:
		return "tici-worker"
	default:
		return "tici"
	}
}

// LogFile implements Instance interface
func (t *TiCIInstance) LogFile() string {
	switch t.role {
	case TiCIRoleMeta:
		return filepath.Join(t.Dir, "tici-meta.log")
	case TiCIRoleWorker:
		return filepath.Join(t.Dir, "tici-worker.log")
	default:
		return filepath.Join(t.Dir, "tici.log")
	}
}

// Cmd returns the process command
func (t *TiCIInstance) Cmd() any {
	if t.process != nil {
		return t.process.Cmd()
	}
	return nil
}

// MetaAddr returns the MetaServer address (only valid for MetaServer instances)
func (t *TiCIInstance) MetaAddr() string {
	if t.role == TiCIRoleMeta {
		return utils.JoinHostPort(AdvertiseHost(t.Host), t.Port)
	}
	return ""
}

// WorkerAddr returns the WorkerNode address (only valid for WorkerNode instances)
func (t *TiCIInstance) WorkerAddr() string {
	if t.role == TiCIRoleWorker {
		return utils.JoinHostPort(AdvertiseHost(t.Host), t.Port)
	}
	return ""
}

// PrepareBinary is a no-op for TiCI since it uses external binaries
func (t *TiCIInstance) PrepareBinary(component, name string, version utils.Version) error {
	// TiCI uses external binaries, no preparation needed
	// But we output the startup message to match other components
	_, _ = colorstr.Printf("[dark_gray]Start %s instance: %s[reset]\n", component, t.BinPath)
	return nil
}

// Terminate terminates the process gracefully
func (t *TiCIInstance) Terminate(sig syscall.Signal) {
	if t.process != nil && t.process.Cmd() != nil && t.process.Cmd().Process != nil {
		_ = syscall.Kill(t.process.Cmd().Process.Pid, sig)
	}
}
