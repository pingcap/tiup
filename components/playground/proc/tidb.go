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
	"strconv"
	"strings"

	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceTiDB is the service ID for TiDB.
	ServiceTiDB ServiceID = "tidb"
	// ServiceTiDBSystem is the service ID for the internal TiDB system service.
	ServiceTiDBSystem ServiceID = "tidb-system"

	// ComponentTiDB is the repository component ID for TiDB.
	ComponentTiDB RepoComponentID = "tidb"
)

func init() {
	RegisterComponentDisplayName(ComponentTiDB, "TiDB")
	RegisterServiceDisplayName(ServiceTiDB, "TiDB")
	RegisterServiceDisplayName(ServiceTiDBSystem, "TiDB System")
}

// TiDBInstance represent a running tidb-server
type TiDBInstance struct {
	ProcessInfo
	ShOpt          SharedOptions
	PDs            []*PDInstance
	KVWorkers      []*TiKVWorkerInstance
	TiProxyCertDir string
	EnableBinlog   bool
}

var _ Process = &TiDBInstance{}

// Prepare builds the TiDB process command.
func (inst *TiDBInstance) Prepare(ctx context.Context) error {
	info := inst.Info()
	configPath := filepath.Join(inst.Dir, "tidb.toml")
	baseConfig, err := inst.getConfig(inst.KVWorkers)
	if err != nil {
		return err
	}
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		baseConfig,
		nil,
	); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.PDs, false)

	args := []string{
		"-P", strconv.Itoa(inst.Port),
		"--store=tikv",
		fmt.Sprintf("--host=%s", inst.Host),
		fmt.Sprintf("--status=%d", inst.StatusPort),
		fmt.Sprintf("--path=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", filepath.Join(inst.Dir, "tidb.log")),
		fmt.Sprintf("--config=%s", configPath),
	}
	if inst.EnableBinlog {
		args = append(args, "--enable-binlog=true")
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}
	return nil
}

// LogFile return the log file name.
func (inst *TiDBInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tidb.log")
}

// Addr return the listen address of TiDB
func (inst *TiDBInstance) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)
}

// WaitReady implements ReadyWaiter.
//
// TiDB is considered ready when its MySQL TCP port is connectable.
func (inst *TiDBInstance) WaitReady(ctx context.Context) error {
	return tcpAddrReady(ctx, inst.Addr(), inst.UpTimeout)
}
