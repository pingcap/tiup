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

	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

type TiFlashRole string

const (
	TiFlashRoleNormal        TiFlashRole = "normal"
	TiFlashRoleDisaggWrite   TiFlashRole = "write"
	TiFlashRoleDisaggCompute TiFlashRole = "compute"
)

type DisaggOptions struct {
	S3Endpoint string `yaml:"s3_endpoint"`
	Bucket     string `yaml:"bucket"`
	AccessKey  string `yaml:"access_key"`
	SecretKey  string `yaml:"secret_key"`
}

// TiFlashInstance represent a running TiFlash
type TiFlashInstance struct {
	instance
	Role            TiFlashRole
	DisaggOpts      DisaggOptions
	TCPPort         int
	ServicePort     int
	ProxyPort       int
	ProxyStatusPort int
	pds             []*PDInstance
	dbs             []*TiDBInstance
	Process
}

// NewTiFlashInstance return a TiFlashInstance
func NewTiFlashInstance(role TiFlashRole, disaggOptions DisaggOptions, binPath, dir, host, configPath string, id int, pds []*PDInstance, dbs []*TiDBInstance, version string) *TiFlashInstance {
	if role != TiFlashRoleNormal && role != TiFlashRoleDisaggWrite && role != TiFlashRoleDisaggCompute {
		panic(fmt.Sprintf("Unknown TiFlash role %s", role))
	}

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
		Role:            role,
		DisaggOpts:      disaggOptions,
		TCPPort:         utils.MustGetFreePort(host, 9000),
		ServicePort:     utils.MustGetFreePort(host, 3930),
		ProxyPort:       utils.MustGetFreePort(host, 20170),
		ProxyStatusPort: utils.MustGetFreePort(host, 20292),
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
func (inst *TiFlashInstance) Start(ctx context.Context, version utils.Version) error {
	if !tidbver.TiFlashPlaygroundNewStartMode(version.String()) {
		return inst.startOld(ctx, version)
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
	}
	args = append(args,
		fmt.Sprintf("--config-file=%s", configPath),
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
