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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

// TiFlashInstance represent a running tikv-server
type TiFlashInstance struct {
	instance
	HTTPPort        int
	ProxyStatusPort int
	ProxyConfigPath string
	pds             []*PDInstance
	dbs             []*TiDBInstance
	cmd             *exec.Cmd
}

// NewTiFlashInstance return a TiFlashInstance
func NewTiFlashInstance(dir, host, configPath string, id int, pds []*PDInstance, dbs []*TiDBInstance) *TiFlashInstance {
	return &TiFlashInstance{
		instance: instance{
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 9000),
			StatusPort: utils.MustGetFreePort(host, 8234),
			ConfigPath: configPath,
		},
		HTTPPort:        utils.MustGetFreePort(host, 8123),
		ProxyStatusPort: utils.MustGetFreePort(host, 20292),
		ProxyConfigPath: configPath,
		pds:             pds,
		dbs:             dbs,
	}
}

const tcpPort = 9000
const httpPort = 8123
const servicePort = 3930
const proxyPort = 20170
const proxyStatusPort = 20292
const metricsPort = 8234

// Start calls set inst.cmd and Start
func (inst *TiFlashInstance) Start(ctx context.Context, version repository.Version, binPath string) error {
	if err := os.MkdirAll(inst.Dir, 0755); err != nil {
		return err
	}
	endpoints := make([]string, 0, len(inst.pds))
	for _, pd := range inst.pds {
		endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", inst.Host, pd.StatusPort))
	}
	tidbStatusAddrs := make([]string, 0, len(inst.dbs))
	for _, db := range inst.dbs {
		tidbStatusAddrs = append(tidbStatusAddrs, fmt.Sprintf("%s:%d", db.Host, uint64(db.StatusPort)))
	}
	wd, err := filepath.Abs(inst.Dir)
	if err != nil {
		return err
	}

	if binPath == "" {
		installedVersion, err := meta.SelectInstalledVersion("tiflash", version)
		if err != nil {
			return err
		}
		dir, err := meta.BinaryPath("tiflash", installedVersion)
		if err != nil {
			return err
		}
		binPath = dir
	}
	// Wait for PD
	time.Sleep(10 * time.Second)
	dirPath := path.Dir(binPath)
	clusterManagerPath := fmt.Sprintf("%s/flash_cluster_manager", dirPath)
	if err := inst.checkConfig(inst.Host, wd, clusterManagerPath, tidbStatusAddrs, endpoints); err != nil {
		return err
	}

	if err := os.Setenv("LD_LIBRARY_PATH", fmt.Sprintf("%s:$LD_LIBRARY_PATH", dirPath)); err != nil {
		return err
	}
	inst.cmd = exec.CommandContext(ctx,
		"tiup", fmt.Sprintf("--binpath=%s", binPath),
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

func (inst *TiFlashInstance) checkConfig(ip, deployDir, clusterManagerPath string, tidbStatusAddrs, endpoints []string) error {
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
	if err := writeTiFlashConfig(cf, tcpPort, httpPort, servicePort, metricsPort,
		ip, deployDir, clusterManagerPath, tidbStatusAddrs, endpoints); err != nil {
		return errors.Trace(err)
	}
	if err := writeTiFlashProxyConfig(cf2, ip, deployDir); err != nil {
		return errors.Trace(err)
	}

	return nil
}
