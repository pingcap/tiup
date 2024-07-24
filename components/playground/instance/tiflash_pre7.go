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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/utils"
)

func getFlashClusterPath(dir string) string {
	return fmt.Sprintf("%s/flash_cluster_manager", dir)
}

type scheduleConfig struct {
	LowSpaceRatio float64 `json:"low-space-ratio"`
}

type replicateMaxReplicaConfig struct {
	MaxReplicas int `json:"max-replicas"`
}

type replicateEnablePlacementRulesConfig struct {
	EnablePlacementRules string `json:"enable-placement-rules"`
}

// startOld is for < 7.1.0. Not maintained any more. Do not introduce new features.
func (inst *TiFlashInstance) startOld(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(inst.pds, false)

	tidbStatusAddrs := make([]string, 0, len(inst.dbs))
	for _, db := range inst.dbs {
		tidbStatusAddrs = append(tidbStatusAddrs, utils.JoinHostPort(AdvertiseHost(db.Host), db.StatusPort))
	}
	wd, err := filepath.Abs(inst.Dir)
	if err != nil {
		return err
	}

	// Wait for PD
	pdClient := api.NewPDClient(ctx, endpoints, 10*time.Second, nil)
	// set low-space-ratio to 1 to avoid low disk space
	lowSpaceRatio, err := json.Marshal(scheduleConfig{
		LowSpaceRatio: 0.99,
	})
	if err != nil {
		return err
	}
	if err = pdClient.UpdateScheduleConfig(bytes.NewBuffer(lowSpaceRatio)); err != nil {
		return err
	}
	// Update maxReplicas before placement rules so that it would not be overwritten
	maxReplicas, err := json.Marshal(replicateMaxReplicaConfig{
		MaxReplicas: 1,
	})
	if err != nil {
		return err
	}
	if err = pdClient.UpdateReplicateConfig(bytes.NewBuffer(maxReplicas)); err != nil {
		return err
	}
	// Set enable-placement-rules to allow TiFlash work properly
	enablePlacementRules, err := json.Marshal(replicateEnablePlacementRulesConfig{
		EnablePlacementRules: "true",
	})
	if err != nil {
		return err
	}
	if err = pdClient.UpdateReplicateConfig(bytes.NewBuffer(enablePlacementRules)); err != nil {
		return err
	}

	dirPath := filepath.Dir(inst.BinPath)
	clusterManagerPath := getFlashClusterPath(dirPath)
	if err = inst.checkConfigOld(wd, clusterManagerPath, version, tidbStatusAddrs, endpoints); err != nil {
		return err
	}

	args := []string{
		"server",
		fmt.Sprintf("--config-file=%s", inst.ConfigPath),
	}
	envs := []string{
		fmt.Sprintf("LD_LIBRARY_PATH=%s:$LD_LIBRARY_PATH", dirPath),
	}
	inst.Process = &process{cmd: PrepareCommand(ctx, inst.BinPath, args, envs, inst.Dir)}

	logIfErr(inst.Process.SetOutputFile(inst.LogFile()))
	return inst.Process.Start()
}

// checkConfigOld is for < 7.1.0. Not maintained any more. Do not introduce new features.
func (inst *TiFlashInstance) checkConfigOld(deployDir, clusterManagerPath string,
	version utils.Version, tidbStatusAddrs, endpoints []string) (err error) {
	if err := utils.MkdirAll(inst.Dir, 0755); err != nil {
		return errors.Trace(err)
	}

	var (
		flashBuf = new(bytes.Buffer)
		proxyBuf = new(bytes.Buffer)

		flashCfgPath = path.Join(inst.Dir, "tiflash.toml")
		proxyCfgPath = path.Join(inst.Dir, "tiflash-learner.toml")
	)

	defer func() {
		if err != nil {
			return
		}
		if err = utils.WriteFile(flashCfgPath, flashBuf.Bytes(), 0644); err != nil {
			return
		}

		if err = utils.WriteFile(proxyCfgPath, proxyBuf.Bytes(), 0644); err != nil {
			return
		}

		inst.ConfigPath = flashCfgPath
	}()

	// Write default config to buffer
	if err := writeTiFlashConfigOld(flashBuf, version, inst.TCPPort, inst.Port, inst.ServicePort, inst.StatusPort,
		inst.Host, deployDir, clusterManagerPath, tidbStatusAddrs, endpoints); err != nil {
		return errors.Trace(err)
	}
	if err := writeTiFlashProxyConfigOld(proxyBuf, version, inst.Host, deployDir,
		inst.ServicePort, inst.ProxyPort, inst.ProxyStatusPort); err != nil {
		return errors.Trace(err)
	}

	if inst.ConfigPath == "" {
		return
	}

	cfg, err := unmarshalConfig(inst.ConfigPath)
	if err != nil {
		return errors.Trace(err)
	}
	proxyPath := getTiFlashProxyConfigPathOld(cfg)
	if proxyPath != "" {
		proxyCfg, err := unmarshalConfig(proxyPath)
		if err != nil {
			return errors.Trace(err)
		}
		err = overwriteBuf(proxyBuf, proxyCfg)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Always use the tiflash proxy config file in the instance directory
	setTiFlashProxyConfigPathOld(cfg, proxyCfgPath)
	return errors.Trace(overwriteBuf(flashBuf, cfg))
}

func overwriteBuf(buf *bytes.Buffer, overwrite map[string]any) (err error) {
	cfg := make(map[string]any)
	if err = toml.Unmarshal(buf.Bytes(), &cfg); err != nil {
		return
	}
	buf.Reset()
	return toml.NewEncoder(buf).Encode(spec.MergeConfig(cfg, overwrite))
}
