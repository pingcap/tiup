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

// TiDBRole is the role of TiDB.
type TiDBRole = string

const (
	// TiDBRoleDefault is the default role of TiDB
	TiDBRoleDefault TiDBRole = "tidb"
	// TiDBRoleSystem is the nextgen role of TiDB
	TiDBRoleSystem TiDBRole = "tidb_system"
)

// TiDBInstance represent a running tidb-server
type TiDBInstance struct {
	instance
	shOpt          SharedOptions
	pds            []*PDInstance
	tiproxyCertDir string
	enableBinlog   bool
}

var _ Instance = &TiDBInstance{}

// NewTiDBInstance return a TiDBInstance
func NewTiDBInstance(shOpt SharedOptions, binPath string, dir, host, configPath string, id, port int, pds []*PDInstance, tiproxyCertDir string, enableBinlog bool, role string) *TiDBInstance {
	if port <= 0 {
		if role == TiDBRoleSystem {
			port = 3000
		} else {
			port = 4000
		}
	}
	return &TiDBInstance{
		shOpt: shOpt,
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, port, shOpt.PortOffset),
			StatusPort: utils.MustGetFreePort("0.0.0.0", 10080, shOpt.PortOffset),
			ConfigPath: configPath,
			role:       role,
		},
		tiproxyCertDir: tiproxyCertDir,
		pds:            pds,
		enableBinlog:   enableBinlog,
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

	return inst.PrepareProcess(ctx, inst.BinPath, args, nil, inst.Dir)
}

// Component returns the package name.
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
