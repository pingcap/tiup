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
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiFlashInstance represent a running TiFlash
type TiFlashInstance struct {
	instance
	TCPPort         int
	ServicePort     int
	ProxyPort       int
	ProxyStatusPort int
	pds             []*PDInstance
	dbs             []*TiDBInstance
	Process
}

// NewTiFlashInstance return a TiFlashInstance
func NewTiFlashInstance(binPath, dir, host, configPath string, id int, pds []*PDInstance, dbs []*TiDBInstance, version string) *TiFlashInstance {
	httpPort := 8123
	if !tidbver.TiFlashNotNeedHTTPPortConfig(version) {
		httpPort = utils.MustGetFreePort(host, httpPort)
	}
	return &TiFlashInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       httpPort,
			StatusPort: utils.MustGetFreePort(host, 8234),
			ConfigPath: configPath,
		},
		TCPPort:         utils.MustGetFreePort(host, 9000),
		ServicePort:     utils.MustGetFreePort(host, 3930),
		ProxyPort:       utils.MustGetFreePort(host, 20170),
		ProxyStatusPort: utils.MustGetFreePort(host, 20292),
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
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ServicePort)
}

// StatusAddrs implements Instance interface.
func (inst *TiFlashInstance) StatusAddrs() (addrs []string) {
	addrs = append(addrs, utils.JoinHostPort(inst.Host, inst.StatusPort))
	addrs = append(addrs, utils.JoinHostPort(inst.Host, inst.ProxyStatusPort))
	return
}

// Start calls set inst.cmd and Start
func (inst *TiFlashInstance) Start(ctx context.Context, version utils.Version) error {
	if tidbver.TiFlashSupportRunWithoutConfig(version.String()) {
		return inst.startViaArgs(ctx, version)
	}
	return inst.startViaConfig(ctx, version)
}

func (inst *TiFlashInstance) startViaArgs(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(inst.pds, false)

	args := []string{
		"server",
	}
	if inst.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config-file=%s", inst.ConfigPath))
	}
	args = append(args,
		"--",
		fmt.Sprintf("--tmp_path=%s", filepath.Join(inst.Dir, "tmp")),
		fmt.Sprintf("--path=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("--listen_host=%s", inst.Host),
		fmt.Sprintf("--tcp_port=%d", inst.TCPPort),
		fmt.Sprintf("--logger.log=%s", inst.LogFile()),
		fmt.Sprintf("--logger.errorlog=%s", filepath.Join(inst.Dir, "tiflash_error.log")),
		fmt.Sprintf("--status.metrics_port=%d", inst.StatusPort),
		fmt.Sprintf("--flash.service_addr=%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ServicePort)),
		fmt.Sprintf("--raft.pd_addr=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--flash.proxy.addr=%s", utils.JoinHostPort(inst.Host, inst.ProxyPort)),
		fmt.Sprintf("--flash.proxy.advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ProxyPort)),
		fmt.Sprintf("--flash.proxy.status-addr=%s", utils.JoinHostPort(inst.Host, inst.ProxyStatusPort)),
		fmt.Sprintf("--flash.proxy.data-dir=%s", filepath.Join(inst.Dir, "proxy_data")),
		fmt.Sprintf("--flash.proxy.log-file=%s", filepath.Join(inst.Dir, "tiflash_tikv.log")),
	)

	var err error
	if inst.BinPath, err = tiupexec.PrepareBinary("tiflash", version, inst.BinPath); err != nil {
		return err
	}
	inst.Process = &process{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}

	logIfErr(inst.Process.SetOutputFile(inst.LogFile()))
	return inst.Process.Start()
}

func (inst *TiFlashInstance) startViaConfig(ctx context.Context, version utils.Version) error {
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

	if inst.BinPath, err = tiupexec.PrepareBinary("tiflash", version, inst.BinPath); err != nil {
		return err
	}

	dirPath := filepath.Dir(inst.BinPath)
	clusterManagerPath := getFlashClusterPath(dirPath)
	if err = inst.checkConfigForStartViaConfig(wd, clusterManagerPath, version, tidbStatusAddrs, endpoints); err != nil {
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
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ServicePort)
}

func (inst *TiFlashInstance) checkConfigForStartViaConfig(deployDir, clusterManagerPath string,
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
	if err := writeTiFlashConfigForStartViaConfig(flashBuf, version, inst.TCPPort, inst.Port, inst.ServicePort, inst.StatusPort,
		inst.Host, deployDir, clusterManagerPath, tidbStatusAddrs, endpoints); err != nil {
		return errors.Trace(err)
	}
	if err := writeTiFlashProxyConfigForStartViaConfig(proxyBuf, version, inst.Host, deployDir,
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
	proxyPath := getTiFlashProxyConfigPath(cfg)
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
	setTiFlashProxyConfigPath(cfg, proxyCfgPath)
	return errors.Trace(overwriteBuf(flashBuf, cfg))
}

func unmarshalConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := make(map[string]any)
	err = toml.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func overwriteBuf(buf *bytes.Buffer, overwrite map[string]any) (err error) {
	cfg := make(map[string]any)
	if err = toml.Unmarshal(buf.Bytes(), &cfg); err != nil {
		return
	}
	buf.Reset()
	return toml.NewEncoder(buf).Encode(spec.MergeConfig(cfg, overwrite))
}
