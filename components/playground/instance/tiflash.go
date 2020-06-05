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
	"runtime"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
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
	cmd             *exec.Cmd
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

// StatusAddrs implements Instance interface.
func (inst *TiFlashInstance) StatusAddrs() (addrs []string) {
	addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Host, inst.StatusPort))
	addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Host, inst.ProxyStatusPort))
	return
}

// Start calls set inst.cmd and Start
func (inst *TiFlashInstance) Start(ctx context.Context, version v0manifest.Version) error {
	if err := os.MkdirAll(inst.Dir, 0755); err != nil {
		return err
	}
	endpoints := make([]string, 0, len(inst.pds))
	for _, pd := range inst.pds {
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", inst.Host, pd.StatusPort))
	}
	tidbStatusAddrs := make([]string, 0, len(inst.dbs))
	for _, db := range inst.dbs {
		tidbStatusAddrs = append(tidbStatusAddrs, fmt.Sprintf("%s:%d", db.Host, uint64(db.StatusPort)))
	}
	wd, err := filepath.Abs(inst.Dir)
	if err != nil {
		return err
	}

	// Wait for PD
	pdClient := api.NewPDClient(endpoints, 10*time.Second, nil)
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

	// TiFlash needs to obtain absolute path of cluster_manager
	if inst.BinPath == "" {
		env, err := environment.InitEnv(repository.Options{
			SkipVersionCheck:  false,
			GOOS:              runtime.GOOS,
			GOARCH:            runtime.GOARCH,
			DisableDecompress: false,
		})
		if err != nil {
			return err
		}
		if version, err = env.GetComponentInstalledVersion("tiflash", version); err != nil {
			return err
		}
		// version may be empty, we will use the latest stable version later in Start cmd.
		if inst.BinPath, err = env.BinaryPath("tiflash", version); err != nil {
			return err
		}
	}

	dirPath := filepath.Dir(inst.BinPath)
	clusterManagerPath := getFlashClusterPath(dirPath)
	if err = inst.checkConfig(wd, clusterManagerPath, tidbStatusAddrs, endpoints); err != nil {
		return err
	}

	if err = os.Setenv("LD_LIBRARY_PATH", fmt.Sprintf("%s:$LD_LIBRARY_PATH", dirPath)); err != nil {
		return err
	}
	inst.cmd = exec.CommandContext(ctx,
		"tiup", fmt.Sprintf("--binpath=%s", inst.BinPath),
		compVersion("tiflash", version),
		"server",
		fmt.Sprintf("--config-file=%s", inst.ConfigPath),
	)
	inst.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, inst.Dir),
	)
	inst.cmd.Stderr = os.Stderr
	inst.cmd.Stdout = os.Stdout
	return inst.cmd.Start()
}

// Wait calls inst.cmd.Wait
func (inst *TiFlashInstance) Wait() error {
	return inst.cmd.Wait()
}

// Pid return the PID of the instance
func (inst *TiFlashInstance) Pid() int {
	return inst.cmd.Process.Pid
}

// Cmd returns the internal Cmd instance
func (inst *TiFlashInstance) Cmd() *exec.Cmd {
	return inst.cmd
}

// StoreAddr return the store address of TiFlash
func (inst *TiFlashInstance) StoreAddr() string {
	return fmt.Sprintf("%s:%d", inst.Host, inst.ServicePort)
}

func (inst *TiFlashInstance) checkConfig(deployDir, clusterManagerPath string, tidbStatusAddrs, endpoints []string) error {
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
	if err := writeTiFlashConfig(cf, inst.TCPPort, inst.Port, inst.ServicePort, inst.StatusPort,
		inst.Host, deployDir, clusterManagerPath, tidbStatusAddrs, endpoints); err != nil {
		return errors.Trace(err)
	}
	if err := writeTiFlashProxyConfig(cf2, inst.Host, deployDir, inst.ServicePort, inst.ProxyPort, inst.ProxyStatusPort); err != nil {
		return errors.Trace(err)
	}

	return nil
}
