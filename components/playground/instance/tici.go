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
	Process

	// TiCI specific fields
	pds  []*PDInstance
	dbs  []*TiDBInstance
	role TiCIRole // Instance role (meta or worker)
}

var _ Instance = &TiCIInstance{}

// NewTiCIMetaInstance creates a TiCI MetaServer instance
func NewTiCIMetaInstance(shOpt SharedOptions, binPath string, dir, host, configPath string, id int, pds []*PDInstance, dbs []*TiDBInstance) *TiCIInstance {
	return NewTiCIInstanceWithRole(shOpt, binPath, dir, host, configPath, id, pds, dbs, TiCIRoleMeta)
}

// NewTiCIWorkerInstance creates a TiCI WorkerNode instance
func NewTiCIWorkerInstance(shOpt SharedOptions, binPath string, dir, host, configPath string, id int, pds []*PDInstance, dbs []*TiDBInstance) *TiCIInstance {
	return NewTiCIInstanceWithRole(shOpt, binPath, dir, host, configPath, id, pds, dbs, TiCIRoleWorker)
}

// NewTiCIInstanceWithRole creates a TiCI instance with specified role
func NewTiCIInstanceWithRole(shOpt SharedOptions, binPath string, dir, host, configPath string, id int, pds []*PDInstance, dbs []*TiDBInstance, role TiCIRole) *TiCIInstance {
	var defaultPort, defaultStatusPort int
	var configFilePath string

	switch role {
	case TiCIRoleMeta:
		// MetaServer default port
		defaultPort = 8500
		defaultStatusPort = 8501
		if configPath != "" {
			configFilePath = filepath.Join(configPath, "test-meta.toml")
		}
	case TiCIRoleWorker:
		// WorkerNode default port
		defaultPort = 8510
		defaultStatusPort = 8511
		if configPath != "" {
			configFilePath = filepath.Join(configPath, "test-worker.toml")
		}
	default:
		panic("invalid TiCI role")
	}

	tici := &TiCIInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, defaultPort, shOpt.PortOffset),
			StatusPort: utils.MustGetFreePort(host, defaultStatusPort, shOpt.PortOffset),
			ConfigPath: configFilePath,
		},
		pds:  pds,
		dbs:  dbs,
		role: role,
	}

	return tici
}

// Start implements Instance interface - starts the appropriate process
func (inst *TiCIInstance) Start(ctx context.Context) error {
	configPath := filepath.Join(inst.Dir, fmt.Sprintf("%s.toml", inst.Component()))
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
	); err != nil {
		return err
	}

	args := []string{
		fmt.Sprintf("--config=%s", configPath),
	}
	inst.Process = &process{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}

	logIfErr(inst.Process.SetOutputFile(inst.LogFile()))
	return inst.Process.Start()
}

func (inst *TiCIInstance) getConfig() map[string]any {
	switch inst.role {
	case TiCIRoleMeta:
		return inst.getMetaConfig()
	case TiCIRoleWorker:
		return inst.getWorkerConfig()
	default:
		return nil // Should not happen
	}
}

// Component implements Process interface
func (inst *TiCIInstance) Component() string {
	switch inst.role {
	case TiCIRoleMeta:
		return "tici-meta"
	case TiCIRoleWorker:
		return "tici-worker"
	default:
		return "tici"
	}
}

// LogFile implements Process interface
func (inst *TiCIInstance) LogFile() string {
	switch inst.role {
	case TiCIRoleMeta:
		return filepath.Join(inst.Dir, "tici-meta.log")
	case TiCIRoleWorker:
		return filepath.Join(inst.Dir, "tici-worker.log")
	default:
		return filepath.Join(inst.Dir, "tici.log")
	}
}

// Addr returns the address for connecting to the TiCI instance.
func (inst *TiCIInstance) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)
}

// StatusAddr returns the status address for the TiCI instance.
func (inst *TiCIInstance) StatusAddr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.StatusPort)
}
