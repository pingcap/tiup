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
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"os"
	"path/filepath"
	"strings"
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
func (c *TiCDC) Start(ctx context.Context, version v0manifest.Version) error {
	if err := os.MkdirAll(c.Dir, 0755); err != nil {
		return err
	}

	var urls []string
	for _, pd := range c.pds {
		urls = append(urls, fmt.Sprintf("http://%s:%d", pd.Host, pd.StatusPort))
	}

	args := []string{
		"server",
		fmt.Sprintf("--addr=%s:%d", c.Host, c.Port),
		fmt.Sprintf("--advertise-addr=%s:%d", advertiseHost(c.Host), c.Port),
		fmt.Sprintf("--pd=%s", strings.Join(urls, ",")),
		fmt.Sprintf("--log-file=%s", c.LogFile()),
	}

	var err error
	if c.Process, err = NewComponentProcess(ctx, c.Dir, c.BinPath, "cdc", version, args...); err != nil {
		return err
	}
	logIfErr(c.Process.SetOutputFile(c.LogFile()))

	return c.Process.Start()
}

// Component return component name.
func (c *TiCDC) Component() string {
	return "ticdc"
}

// LogFile return the log file.
func (c *TiCDC) LogFile() string {
	return filepath.Join(c.Dir, "ticdc.log")
}
