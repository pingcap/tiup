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

package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	ServiceTiKV ServiceID = "tikv"

	ComponentTiKV RepoComponentID = "tikv"
)

// TiKVInstance represent a running tikv-server
type TiKVInstance struct {
	ProcessInfo
	ShOpt SharedOptions
	PDs   []*PDInstance
	TSOs  []*PDInstance
}

var _ Process = &TiKVInstance{}

func init() {
	RegisterComponentDisplayName(ComponentTiKV, "TiKV")
	RegisterServiceDisplayName(ServiceTiKV, "TiKV")
}

// Addr return the address of tikv.
func (inst *TiKVInstance) Addr() string {
	return utils.JoinHostPort(inst.Host, inst.Port)
}

// Prepare builds the TiKV process command.
func (inst *TiKVInstance) Prepare(ctx context.Context) error {
	info := inst.Info()

	configPath := filepath.Join(inst.Dir, "tikv.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
	); err != nil {
		return err
	}

	// Need to check tso status
	if inst.ShOpt.PDMode == "ms" {
		var tsoEnds []string
		for _, pd := range inst.TSOs {
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

	endpoints := pdEndpoints(inst.PDs, true)
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
	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, envs, inst.Dir)}
	return nil
}

// LogFile return the log file name.
func (inst *TiKVInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tikv.log")
}

// StoreAddr return the store address of TiKV
func (inst *TiKVInstance) StoreAddr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)
}
