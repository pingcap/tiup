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

package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceTiKVWorker is the service ID for TiKV-worker.
	ServiceTiKVWorker ServiceID = "tikv-worker"

	// ComponentTiKVWorker is the repository component ID for TiKV-worker.
	ComponentTiKVWorker RepoComponentID = "tikv-worker"
)

// ResolveTiKVWorkerBinPath resolves the tikv-worker binary path when a
// tikv-server path is provided.
func ResolveTiKVWorkerBinPath(binPath string) string {
	if !strings.HasSuffix(binPath, "tikv-server") {
		return binPath
	}
	dir := filepath.Dir(binPath)
	return filepath.Join(dir, "tikv-worker")
}

// TiKVWorkerInstance represent a running TiKVWorker instance.
type TiKVWorkerInstance struct {
	ProcessInfo
	ShOpt SharedOptions
	PDs   []*PDInstance
}

var _ Process = &TiKVWorkerInstance{}

func init() {
	RegisterComponentDisplayName(ComponentTiKVWorker, "TiKV Worker")
	RegisterServiceDisplayName(ServiceTiKVWorker, "TiKV Worker")
}

// Addr return the address of TiKVWorker.
func (inst *TiKVWorkerInstance) Addr() string {
	return utils.JoinHostPort(inst.Host, inst.Port)
}

// Prepare builds the TiKV worker process command.
func (inst *TiKVWorkerInstance) Prepare(ctx context.Context) error {
	info := inst.Info()
	if inst.ShOpt.PDMode == "ms" {
		return errors.New("tikv_worker does not support ms pd mode")
	}
	switch inst.ShOpt.Mode {
	case ModeCSE, ModeNextGen:
	default:
		return errors.Errorf("tikv_worker does not support %s mode", inst.ShOpt.Mode)
	}

	configPath := filepath.Join(inst.Dir, "tikv_worker.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
		nil,
	); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.PDs, true)
	args := []string{
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(inst.Host, inst.Port)),
		fmt.Sprintf("--log-file=%s", inst.LogFile()),
		fmt.Sprintf("--pd-endpoints=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--config=%s", configPath),
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}
	return nil
}

// LogFile return the log file name.
func (inst *TiKVWorkerInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tikv_worker.log")
}

func (inst *TiKVWorkerInstance) getConfig() map[string]any {
	config := make(map[string]any)
	applyS3DFSConfig(config, inst.ShOpt.CSE, "tikv")
	config["raft-engine.enabled"] = false
	config["schema-manager.dir"] = filepath.Join(inst.Dir, "schemas")
	config["schema-manager.schema-refresh-threshold"] = 1
	config["schema-manager.enabled"] = true
	config["schema-manager.keyspace-refresh-interval"] = "10s"

	return config
}
