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
	"path"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiKVInstance represent a running tikv-server
type TiKVInstance struct {
	instance
	pds []*PDInstance
	Process
}

// NewTiKVInstance return a TiKVInstance
func NewTiKVInstance(binPath string, dir, host, configPath string, id int, pds []*PDInstance) *TiKVInstance {
	return &TiKVInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 20160),
			StatusPort: utils.MustGetFreePort(host, 20180),
			ConfigPath: configPath,
		},
		pds: pds,
	}
}

// Addr return the address of tikv.
func (inst *TiKVInstance) Addr() string {
	return fmt.Sprintf("%s:%d", inst.Host, inst.Port)
}

// Start calls set inst.cmd and Start
func (inst *TiKVInstance) Start(ctx context.Context, version utils.Version) error {
	if err := inst.checkConfig(); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.pds, true)
	args := []string{
		fmt.Sprintf("--addr=%s:%d", inst.Host, inst.Port),
		fmt.Sprintf("--advertise-addr=%s:%d", AdvertiseHost(inst.Host), inst.Port),
		fmt.Sprintf("--status-addr=%s:%d", inst.Host, inst.StatusPort),
		fmt.Sprintf("--pd=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--config=%s", inst.ConfigPath),
		fmt.Sprintf("--data-dir=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("--log-file=%s", inst.LogFile()),
	}

	envs := []string{"MALLOC_CONF=prof:true,prof_active:false"}
	var err error
	if inst.BinPath, err = tiupexec.PrepareBinary("tikv", version, inst.BinPath); err != nil {
		return err
	}
	inst.Process = &process{cmd: PrepareCommand(ctx, inst.BinPath, args, envs, inst.Dir)}

	logIfErr(inst.Process.SetOutputFile(inst.LogFile()))
	return inst.Process.Start()
}

// Component return the component name.
func (inst *TiKVInstance) Component() string {
	return "tikv"
}

// LogFile return the log file name.
func (inst *TiKVInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tikv.log")
}

// StoreAddr return the store address of TiKV
func (inst *TiKVInstance) StoreAddr() string {
	return fmt.Sprintf("%s:%d", AdvertiseHost(inst.Host), inst.Port)
}

func (inst *TiKVInstance) checkConfig() error {
	if err := os.MkdirAll(inst.Dir, 0755); err != nil {
		return err
	}
	if inst.ConfigPath == "" {
		inst.ConfigPath = path.Join(inst.Dir, "tikv.toml")
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
	if err := writeTiKVConfig(cf); err != nil {
		return errors.Trace(err)
	}

	return nil
}
