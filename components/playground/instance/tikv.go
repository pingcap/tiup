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
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiKVInstance represent a running tikv-server
type TiKVInstance struct {
	instance
	pds  []*PDInstance
	tsos []*PDInstance
	Process
	mode       string
	cseOpts    CSEOptions
	isPDMSMode bool
}

// NewTiKVInstance return a TiKVInstance
func NewTiKVInstance(binPath string, dir, host, configPath string, portOffset int, id int, port int, pds []*PDInstance, tsos []*PDInstance, mode string, cseOptions CSEOptions, isPDMSMode bool) *TiKVInstance {
	if port <= 0 {
		port = 20160
	}
	return &TiKVInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, port, portOffset),
			StatusPort: utils.MustGetFreePort(host, 20180, portOffset),
			ConfigPath: configPath,
		},
		pds:        pds,
		tsos:       tsos,
		mode:       mode,
		cseOpts:    cseOptions,
		isPDMSMode: isPDMSMode,
	}
}

// Addr return the address of tikv.
func (inst *TiKVInstance) Addr() string {
	return utils.JoinHostPort(inst.Host, inst.Port)
}

// Start calls set inst.cmd and Start
func (inst *TiKVInstance) Start(ctx context.Context) error {
	configPath := filepath.Join(inst.Dir, "tikv.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
	); err != nil {
		return err
	}

	// Need to check tso status
	if inst.isPDMSMode {
		var tsoEnds []string
		for _, pd := range inst.tsos {
			tsoEnds = append(tsoEnds, fmt.Sprintf("%s:%d", AdvertiseHost(pd.Host), pd.StatusPort))
		}
		pdcli := api.NewPDClient(ctx,
			tsoEnds, 10*time.Second, nil,
		)
		if err := pdcli.CheckTSOHealth(&utils.RetryOption{
			Delay:   time.Second * 5,
			Timeout: time.Second * 300,
		}); err != nil {
			return err
		}
	}

	endpoints := pdEndpoints(inst.pds, true)
	args := []string{
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(inst.Host, inst.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)),
		fmt.Sprintf("--status-addr=%s", utils.JoinHostPort(inst.Host, inst.StatusPort)),
		fmt.Sprintf("--pd-endpoints=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--config=%s", configPath),
		fmt.Sprintf("--data-dir=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("--log-file=%s", inst.LogFile()),
	}

	envs := []string{"MALLOC_CONF=prof:true,prof_active:false"}
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
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)
}
