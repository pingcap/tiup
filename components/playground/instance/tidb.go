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
	"strconv"
	"strings"

	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiDBInstance represent a running tidb-server
type TiDBInstance struct {
	instance
	pds []*PDInstance
	Process
	enableBinlog bool
}

// NewTiDBInstance return a TiDBInstance
func NewTiDBInstance(binPath string, dir, host, configPath string, id, port int, pds []*PDInstance, enableBinlog bool) *TiDBInstance {
	if port <= 0 {
		port = 4000
	}
	return &TiDBInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, port),
			StatusPort: utils.MustGetFreePort("0.0.0.0", 10080),
			ConfigPath: configPath,
		},
		pds:          pds,
		enableBinlog: enableBinlog,
	}
}

// Start calls set inst.cmd and Start
func (inst *TiDBInstance) Start(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(inst.pds, false)

	args := []string{
		"-P", strconv.Itoa(inst.Port),
		"--store=tikv",
		fmt.Sprintf("--host=%s", inst.Host),
		fmt.Sprintf("--status=%d", inst.StatusPort),
		fmt.Sprintf("--path=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", filepath.Join(inst.Dir, "tidb.log")),
	}
	if inst.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", inst.ConfigPath))
	}
	if inst.enableBinlog {
		args = append(args, "--enable-binlog=true")
	}

	var err error
	if inst.BinPath, err = tiupexec.PrepareBinary("tidb", version, inst.BinPath); err != nil {
		return err
	}
	inst.Process = &process{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}

	logIfErr(inst.Process.SetOutputFile(inst.LogFile()))
	return inst.Process.Start()
}

// Component return the component name.
func (inst *TiDBInstance) Component() string {
	return "tidb"
}

// LogFile return the log file name.
func (inst *TiDBInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tidb.log")
}

// Addr return the listen address of TiDB
func (inst *TiDBInstance) Addr() string {
	return fmt.Sprintf("%s:%d", AdvertiseHost(inst.Host), inst.Port)
}
