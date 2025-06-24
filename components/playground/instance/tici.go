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
	pds       []*PDInstance
	ticiDir   string   // TiCI project directory
	configDir string   // Configuration directory
	role      TiCIRole // Instance role (meta or worker)

	// Process - only one process per instance
	process *process
}

var _ Instance = &TiCIInstance{}

// NewTiCIMetaInstance creates a TiCI MetaServer instance
func NewTiCIMetaInstance(shOpt SharedOptions, baseDir, host string, id int, pds []*PDInstance,
	ticiDir, configDir string) *TiCIInstance {
	return NewTiCIInstanceWithRole(shOpt, baseDir, host, id, pds, ticiDir, configDir, TiCIRoleMeta)
}

// NewTiCIWorkerInstance creates a TiCI WorkerNode instance
func NewTiCIWorkerInstance(shOpt SharedOptions, baseDir, host string, id int, pds []*PDInstance,
	ticiDir, configDir string) *TiCIInstance {
	return NewTiCIInstanceWithRole(shOpt, baseDir, host, id, pds, ticiDir, configDir, TiCIRoleWorker)
}

// NewTiCIInstanceWithRole creates a TiCI instance with specified role
func NewTiCIInstanceWithRole(shOpt SharedOptions, baseDir, host string, id int, pds []*PDInstance,
	ticiDir, configDir string, role TiCIRole) *TiCIInstance {
	var componentSuffix string
	var defaultPort int

	switch role {
	case TiCIRoleMeta:
		componentSuffix = "meta"
		defaultPort = 8500 // MetaServer default port
	case TiCIRoleWorker:
		componentSuffix = "worker"
		defaultPort = 8501 // WorkerNode default port
	default:
		panic("invalid TiCI role")
	}

	dir := filepath.Join(baseDir, fmt.Sprintf("tici-%s-%d", componentSuffix, id))

	tici := &TiCIInstance{
		instance: instance{
			BinPath:    ticiDir, // TiCI project directory
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, defaultPort, shOpt.PortOffset),
			StatusPort: utils.MustGetFreePort(host, defaultPort+1, shOpt.PortOffset),
			ConfigPath: configDir,
		},
		pds:       pds,
		ticiDir:   ticiDir,
		configDir: configDir,
		role:      role,
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

	switch t.role {
	case TiCIRoleMeta:
		return t.startMetaServer(ctx)
	case TiCIRoleWorker:
		return t.startWorkerNode(ctx)
	default:
		return fmt.Errorf("invalid TiCI role: %v", t.role)
	}
}

func (t *TiCIInstance) startMetaServer(ctx context.Context) error {
	args := []string{}

	// Use default or provided config path
	metaConfigPath := filepath.Join(t.ticiDir, "ci", "test-meta-config.toml")
	if t.configDir != "" {
		metaConfigPath = filepath.Join(t.configDir, "test-meta-config.toml")
	}
	args = append(args, fmt.Sprintf("--config=%s", metaConfigPath))

	// MetaServer binary path
	metaBinPath := filepath.Join(t.ticiDir, "target", "debug", "meta_service_server")
	t.process = &process{cmd: PrepareCommand(ctx, metaBinPath, args, nil, t.ticiDir)}

	// Set up logging
	logFile := filepath.Join(t.Dir, "tici-meta.log")
	logIfErr(t.process.SetOutputFile(logFile))

	return t.process.Start()
}

func (t *TiCIInstance) startWorkerNode(ctx context.Context) error {
	args := []string{}

	// Use default or provided config path
	workerConfigPath := filepath.Join(t.ticiDir, "ci", "test-writer.toml")
	if t.configDir != "" {
		workerConfigPath = filepath.Join(t.configDir, "test-writer.toml")
	}
	args = append(args, fmt.Sprintf("--config=%s", workerConfigPath))

	// WorkerNode binary path
	workerBinPath := filepath.Join(t.ticiDir, "target", "debug", "worker_node_server")
	t.process = &process{cmd: PrepareCommand(ctx, workerBinPath, args, nil, t.ticiDir)}

	// Set up logging
	logFile := filepath.Join(t.Dir, "tici-worker.log")
	logIfErr(t.process.SetOutputFile(logFile))

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
	colorstr.Printf("[dark_gray]Start %s instance: %s[reset]\n", component, t.ticiDir)
	return nil
}

// Terminate terminates the process gracefully
func (t *TiCIInstance) Terminate(sig syscall.Signal) {
	if t.process != nil && t.process.Cmd() != nil && t.process.Cmd().Process != nil {
		_ = syscall.Kill(t.process.Cmd().Process.Pid, sig)
	}
}
