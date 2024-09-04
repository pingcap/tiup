// Copyright 2022 PingCAP, Inc.
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

	"github.com/pingcap/tiup/pkg/utils"
)

// TiKVCDC represent a TiKV-CDC instance.
type TiKVCDC struct {
	instance
	pds []*PDInstance
	Process
}

var _ Instance = &TiKVCDC{}

// NewTiKVCDC create a TiKVCDC instance.
func NewTiKVCDC(binPath string, dir, host, configPath string, portOffset int, id int, pds []*PDInstance) *TiKVCDC {
	tikvCdc := &TiKVCDC{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 8600, portOffset),
			ConfigPath: configPath,
		},
		pds: pds,
	}
	tikvCdc.StatusPort = tikvCdc.Port
	return tikvCdc
}

// Start implements Instance interface.
func (c *TiKVCDC) Start(ctx context.Context) error {
	endpoints := pdEndpoints(c.pds, true)

	args := []string{
		"server",
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(c.Host, c.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(c.Host), c.Port)),
		fmt.Sprintf("--pd=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", c.LogFile()),
		fmt.Sprintf("--data-dir=%s", filepath.Join(c.Dir, "data")),
	}
	if c.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", c.ConfigPath))
	}

	c.Process = &process{cmd: PrepareCommand(ctx, c.BinPath, args, nil, c.Dir)}

	logIfErr(c.Process.SetOutputFile(c.LogFile()))
	return c.Process.Start()
}

// Component return component name.
func (c *TiKVCDC) Component() string {
	return "tikv-cdc"
}

// LogFile return the log file.
func (c *TiKVCDC) LogFile() string {
	return filepath.Join(c.Dir, "tikv_cdc.log")
}
