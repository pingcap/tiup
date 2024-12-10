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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiFlashRole is the role of TiFlash.
type TiFlashRole string

const (
	// TiFlashRoleNormal is used when TiFlash is not in disaggregated mode.
	TiFlashRoleNormal TiFlashRole = "normal"

	// TiFlashRoleDisaggWrite is used when TiFlash is in disaggregated mode and is the write node.
	TiFlashRoleDisaggWrite TiFlashRole = "write"
	// TiFlashRoleDisaggCompute is used when TiFlash is in disaggregated mode and is the compute node.
	TiFlashRoleDisaggCompute TiFlashRole = "compute"
)

// TiFlashInstance represent a running TiFlash
type TiFlashInstance struct {
	instance
	Role            TiFlashRole
	cseOpts         CSEOptions
	TCPPort         int
	ServicePort     int
	ProxyPort       int
	ProxyStatusPort int
	pds             []*PDInstance
	dbs             []*TiDBInstance
	Process
}

// NewTiFlashInstance return a TiFlashInstance
func NewTiFlashInstance(role TiFlashRole, cseOptions CSEOptions, binPath, dir, host, configPath string, portOffset int, id int, pds []*PDInstance, dbs []*TiDBInstance, version string) *TiFlashInstance {
	if role != TiFlashRoleNormal && role != TiFlashRoleDisaggWrite && role != TiFlashRoleDisaggCompute {
		panic(fmt.Sprintf("Unknown TiFlash role %s", role))
	}

	httpPort := 8123
	if !tidbver.TiFlashNotNeedHTTPPortConfig(version) {
		httpPort = utils.MustGetFreePort(host, httpPort, portOffset)
	}
	return &TiFlashInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       httpPort,
			StatusPort: utils.MustGetFreePort(host, 8234, portOffset),
			ConfigPath: configPath,
		},
		Role:            role,
		cseOpts:         cseOptions,
		TCPPort:         utils.MustGetFreePort(host, 9100, portOffset), // 9000 for default object store port
		ServicePort:     utils.MustGetFreePort(host, 3930, portOffset),
		ProxyPort:       utils.MustGetFreePort(host, 20170, portOffset),
		ProxyStatusPort: utils.MustGetFreePort(host, 20292, portOffset),
		pds:             pds,
		dbs:             dbs,
	}
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
func (inst *TiFlashInstance) Start(ctx context.Context) error {
	if !tidbver.TiFlashPlaygroundNewStartMode(inst.Version.String()) {
		return inst.startOld(ctx, inst.Version)
	}

	proxyConfigPath := filepath.Join(inst.Dir, "tiflash_proxy.toml")
	if err := prepareConfig(
		proxyConfigPath,
		"",
		inst.getProxyConfig(),
	); err != nil {
		return err
	}

	configPath := filepath.Join(inst.Dir, "tiflash.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
	); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.pds, false)

	args := []string{
		"server",
		fmt.Sprintf("--config-file=%s", configPath),
		"--",
	}
	runtimeConfig := [][]string{
		{"path", filepath.Join(inst.Dir, "data")},
		{"listen_host", inst.Host},
		{"logger.log", inst.LogFile()},
		{"logger.errorlog", filepath.Join(inst.Dir, "tiflash_error.log")},
		{"status.metrics_port", fmt.Sprintf("%d", inst.StatusPort)},
		{"flash.service_addr", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ServicePort)},
		{"raft.pd_addr", strings.Join(endpoints, ",")},
		{"flash.proxy.addr", utils.JoinHostPort(inst.Host, inst.ProxyPort)},
		{"flash.proxy.advertise-addr", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ProxyPort)},
		{"flash.proxy.status-addr", utils.JoinHostPort(inst.Host, inst.ProxyStatusPort)},
		{"flash.proxy.data-dir", filepath.Join(inst.Dir, "proxy_data")},
		{"flash.proxy.log-file", filepath.Join(inst.Dir, "tiflash_tikv.log")},
	}
	userConfig, err := unmarshalConfig(configPath)
	if err != nil {
		return errors.Trace(err)
	}
	for _, arg := range runtimeConfig {
		// if user has set the config, skip it
		if !isKeyPresentInMap(userConfig, arg[0]) {
			args = append(args, fmt.Sprintf("--%s=%s", arg[0], arg[1]))
		}
	}

	inst.Process = &process{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}

	logIfErr(inst.Process.SetOutputFile(inst.LogFile()))
	return inst.Process.Start()
}

func isKeyPresentInMap(m map[string]any, key string) bool {
	keys := strings.Split(key, ".")
	currentMap := m

	for i := 0; i < len(keys); i++ {
		if _, ok := currentMap[keys[i]]; !ok {
			return false
		}

		// If the current value is a nested map, update the current map to the nested map
		if innerMap, ok := currentMap[keys[i]].(map[string]any); ok {
			currentMap = innerMap
		}
	}

	return true
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
