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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiFlashInstance represent a running TiFlash
type TiFlashInstance struct {
	instance
	TCPPort         int
	ServicePort     int
	ProxyPort       int
	ProxyStatusPort int
	ProxyConfigPath string
	pds             []*PDInstance
	dbs             []*TiDBInstance
	Process
}

// NewTiFlashInstance return a TiFlashInstance
func NewTiFlashInstance(binPath, dir, host, configPath string, id int, pds []*PDInstance, dbs []*TiDBInstance) *TiFlashInstance {
	return &TiFlashInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 8123),
			StatusPort: utils.MustGetFreePort(host, 8234),
			ConfigPath: configPath,
		},
		TCPPort:         utils.MustGetFreePort(host, 9000),
		ServicePort:     utils.MustGetFreePort(host, 3930),
		ProxyPort:       utils.MustGetFreePort(host, 20170),
		ProxyStatusPort: utils.MustGetFreePort(host, 20292),
		ProxyConfigPath: configPath,
		pds:             pds,
		dbs:             dbs,
	}
}

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

// Addr return the address of tiflash
func (inst *TiFlashInstance) Addr() string {
	return fmt.Sprintf("%s:%d", inst.Host, inst.ServicePort)
}

// StatusAddrs implements Instance interface.
func (inst *TiFlashInstance) StatusAddrs() (addrs []string) {
	addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Host, inst.StatusPort))
	addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Host, inst.ProxyStatusPort))
	return
}

// Start calls set inst.cmd and Start
func (inst *TiFlashInstance) Start(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(inst.pds, false)

	tidbStatusAddrs := make([]string, 0, len(inst.dbs))
	for _, db := range inst.dbs {
		tidbStatusAddrs = append(tidbStatusAddrs, fmt.Sprintf("%s:%d", AdvertiseHost(db.Host), uint64(db.StatusPort)))
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

	if inst.BinPath, err = tiupexec.PrepareBinary("tiflash", version, inst.BinPath); err != nil {
		return err
	}

	dirPath := filepath.Dir(inst.BinPath)
	clusterManagerPath := getFlashClusterPath(dirPath)
	if err = inst.checkConfig(wd, clusterManagerPath, version, tidbStatusAddrs, endpoints); err != nil {
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

// Component return the component name.
func (inst *TiFlashInstance) Component() string {
	return "tiflash"
}

// LogFile return the log file name.
func (inst *TiFlashInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tiflash.log")
}

// Cmd returns the internal Cmd instance
func (inst *TiFlashInstance) Cmd() *exec.Cmd {
	return inst.Process.Cmd()
}

// StoreAddr return the store address of TiFlash
func (inst *TiFlashInstance) StoreAddr() string {
	return fmt.Sprintf("%s:%d", AdvertiseHost(inst.Host), inst.ServicePort)
}

func (inst *TiFlashInstance) checkConfig(deployDir, clusterManagerPath string, version utils.Version, tidbStatusAddrs, endpoints []string) error {
	if err := os.MkdirAll(inst.Dir, 0755); err != nil {
		return errors.Trace(err)
	}
	if inst.ConfigPath == "" {
		inst.ConfigPath = path.Join(inst.Dir, "tiflash.toml")
	}
	if inst.ProxyConfigPath == "" {
		inst.ProxyConfigPath = path.Join(inst.Dir, "tiflash-learner.toml")
	}

	_, err := os.Stat(inst.ConfigPath)
	if err == nil || os.IsExist(err) {
		return nil
	}
	if !os.IsNotExist(err) {
		return errors.Trace(err)
	}
	cf, err := os.Create(inst.ConfigPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer cf.Close()

	_, err = os.Stat(inst.ProxyConfigPath)
	if err == nil || os.IsExist(err) {
		return nil
	}
	if !os.IsNotExist(err) {
		return errors.Trace(err)
	}
	cf2, err := os.Create(inst.ProxyConfigPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer cf2.Close()
	if err := writeTiFlashConfig(cf, version, inst.TCPPort, inst.Port, inst.ServicePort, inst.StatusPort,
		inst.Host, deployDir, clusterManagerPath, tidbStatusAddrs, endpoints); err != nil {
		return errors.Trace(err)
	}
	if err := writeTiFlashProxyConfig(cf2, version, inst.Host, deployDir, inst.ServicePort, inst.ProxyPort, inst.ProxyStatusPort); err != nil {
		return errors.Trace(err)
	}

	return nil
}
