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

	"github.com/pingcap/tiup/pkg/utils"
)

// TiDBInstance represent a running tidb-server
type TiDBInstance struct {
	instance
	pds []*PDInstance
	Process
	tiproxyCertDir string
	enableBinlog   bool
	mode           string
}

// NewTiDBInstance return a TiDBInstance
func NewTiDBInstance(binPath string, dir, host, configPath string, portOffset int, id, port int, pds []*PDInstance, tiproxyCertDir string, enableBinlog bool, mode string) *TiDBInstance {
	if port <= 0 {
		port = 4000
	}
	return &TiDBInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, port, portOffset),
			StatusPort: utils.MustGetFreePort("0.0.0.0", 10080, portOffset),
			ConfigPath: configPath,
		},
		tiproxyCertDir: tiproxyCertDir,
		pds:            pds,
		enableBinlog:   enableBinlog,
		mode:           mode,
	}
}

// Start calls set inst.cmd and Start
func (inst *TiDBInstance) Start(ctx context.Context) error {
	configPath := filepath.Join(inst.Dir, "tidb.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
	); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.pds, false)

	args := []string{
		"-P", strconv.Itoa(inst.Port),
		"--store=tikv",
		fmt.Sprintf("--host=%s", inst.Host),
		fmt.Sprintf("--status=%d", inst.StatusPort),
		fmt.Sprintf("--path=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", filepath.Join(inst.Dir, "tidb.log")),
		fmt.Sprintf("--config=%s", configPath),
	}
	if inst.enableBinlog {
		args = append(args, "--enable-binlog=true")
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
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)
}
