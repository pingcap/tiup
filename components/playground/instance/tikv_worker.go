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
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

// resolveTiKVWorkerBinPath resolves the tikv-worker binary path when tikv-server path is provided.
func resolveTiKVWorkerBinPath(binPath string) string {
	if !strings.HasSuffix(binPath, "tikv-server") {
		return binPath
	}
	dir := filepath.Dir(binPath)
	return filepath.Join(dir, "tikv-worker")
}

// TiKVWorkerInstance represent a running TiKVWorker instance.
type TiKVWorkerInstance struct {
	instance
	shOpt SharedOptions
	pds   []*PDInstance
}

var _ Instance = &TiKVWorkerInstance{}

// NewTiKVWorkerInstance creates a new TiKVWorker instance.
func NewTiKVWorkerInstance(shOpt SharedOptions, binPath string, dir, host, configPath string, id int, port int, pds []*PDInstance) *TiKVWorkerInstance {
	if port <= 0 {
		port = 19000
	}
	return &TiKVWorkerInstance{
		shOpt: shOpt,
		instance: instance{
			BinPath:    resolveTiKVWorkerBinPath(binPath),
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, port, shOpt.PortOffset),
			ConfigPath: configPath,
			role:       "tikv_worker",
		},
		pds: pds,
	}
}

// Addr return the address of TiKVWorker.
func (inst *TiKVWorkerInstance) Addr() string {
	return utils.JoinHostPort(inst.Host, inst.Port)
}

// Start calls set inst.cmd and Start
func (inst *TiKVWorkerInstance) Start(ctx context.Context) error {
	if inst.shOpt.PDMode == "ms" {
		return errors.New("tikv_worker does not support ms pd mode")
	}
	switch inst.shOpt.Mode {
	case ModeCSE, ModeNextGen, ModeNextGenFTS:
	default:
		return errors.Errorf("tikv_worker does not support %s mode", inst.shOpt.Mode)
	}

	configPath := filepath.Join(inst.Dir, "tikv_worker.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
	); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.pds, true)
	args := []string{
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(inst.Host, inst.Port)),
		fmt.Sprintf("--log-file=%s", inst.LogFile()),
		fmt.Sprintf("--pd-endpoints=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--config=%s", configPath),
	}

	return inst.PrepareProcess(ctx, inst.BinPath, args, nil, inst.Dir)
}

// LogFile return the log file name.
func (inst *TiKVWorkerInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tikv_worker.log")
}

// Component return the binary name.
func (inst *TiKVWorkerInstance) Component() string {
	if IsNGMode(inst.shOpt.Mode) {
		return "tikv-worker"
	}
	return "tikv"
}

func (inst *TiKVWorkerInstance) getConfig() map[string]any {
	config := make(map[string]any)
	config["dfs.prefix"] = "tikv"
	config["dfs.s3-endpoint"] = inst.shOpt.S3.Endpoint
	config["dfs.s3-key-id"] = inst.shOpt.S3.AccessKey
	config["dfs.s3-secret-key"] = inst.shOpt.S3.SecretKey
	config["dfs.s3-bucket"] = inst.shOpt.S3.Bucket
	config["dfs.s3-region"] = "local"
	config["raft-engine.enabled"] = false
	config["schema-manager.dir"] = filepath.Join(inst.Dir, "schemas")
	config["schema-manager.schema-refresh-threshold"] = 1
	config["schema-manager.enabled"] = true
	config["schema-manager.keyspace-refresh-interval"] = "10s"

	return config
}
