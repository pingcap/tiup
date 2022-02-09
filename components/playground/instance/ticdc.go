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

	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

// TiCDC represent a ticdc instance.
type TiCDC struct {
	instance
	pds []*PDInstance
	Process
}

var _ Instance = &TiCDC{}

// NewTiCDC create a TiCDC instance.
func NewTiCDC(binPath string, dir, host, configPath string, id int, pds []*PDInstance) *TiCDC {
	ticdc := &TiCDC{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 8300),
			ConfigPath: configPath,
		},
		pds: pds,
	}
	ticdc.StatusPort = ticdc.Port
	return ticdc
}

// Start implements Instance interface.
func (c *TiCDC) Start(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(c.pds, true)

	args := []string{
		"server",
		fmt.Sprintf("--addr=%s:%d", c.Host, c.Port),
		fmt.Sprintf("--advertise-addr=%s:%d", AdvertiseHost(c.Host), c.Port),
		fmt.Sprintf("--pd=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", c.LogFile()),
	}
	clusterVersion := string(version)
	if semver.Compare(clusterVersion, "v4.0.13") >= 0 {
		if (semver.Major(clusterVersion) == "v4" && semver.Compare(clusterVersion, "v4.0.14") >= 0) ||
			(semver.Major(clusterVersion) == "v5" && semver.Compare(clusterVersion, "v5.0.3") >= 0) {
			args = append(args, fmt.Sprintf("--data-dir=%s", filepath.Join(c.Dir, "data")))
		} else {
			args = append(args, fmt.Sprintf("--sort-dir=%s/tmp/sorter", filepath.Join(c.Dir, "data")))
		}
	}

	var err error
	if c.BinPath, err = tiupexec.PrepareBinary("cdc", version, c.BinPath); err != nil {
		return err
	}
	c.Process = &process{cmd: PrepareCommand(ctx, c.BinPath, args, nil, c.Dir)}

	logIfErr(c.Process.SetOutputFile(c.LogFile()))
	return c.Process.Start()
}

// Component return component name.
func (c *TiCDC) Component() string {
	return "cdc"
}

// LogFile return the log file.
func (c *TiCDC) LogFile() string {
	return filepath.Join(c.Dir, "ticdc.log")
}
