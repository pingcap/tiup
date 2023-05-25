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
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiProxy represent a ticdc instance.
type TiProxy struct {
	instance
	pds []*PDInstance
	Process
}

var _ Instance = &TiProxy{}

// NewTiProxy create a TiProxy instance.
func NewTiProxy(binPath string, dir, host, configPath string, id int, port int, pds []*PDInstance) *TiProxy {
	if port <= 0 {
		port = 4000
	}
	tiproxy := &TiProxy{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, port),
			StatusPort: utils.MustGetFreePort(host, 3080),
			ConfigPath: configPath,
		},
		pds: pds,
	}
	return tiproxy
}

// Start implements Instance interface.
func (c *TiProxy) Start(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(c.pds, true)

	configPath := filepath.Join(c.Dir, "config", "proxy.toml")
	if c.ConfigPath != "" {
		configPath = c.ConfigPath
	}

	configContent := ""
	if b, err := os.ReadFile(configPath); err == nil {
		configContent = string(b)
	}

	config, err := toml.Load(configContent)
	if err != nil {
		return err
	}

	config.Set("proxy.pd-addrs", strings.Join(endpoints, ","))
	config.Set("proxy.addr", utils.JoinHostPort(c.Host, c.Port))
	config.Set("proxy.require-backend-tls", false)
	config.Set("api.addr", utils.JoinHostPort(c.Host, c.StatusPort))

	b, err := config.Marshal()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, b, 0644); err != nil {
		return err
	}

	args := []string{
		fmt.Sprintf("--config=%s", configPath),
	}

	if c.BinPath, err = tiupexec.PrepareBinary("tiproxy", version, c.BinPath); err != nil {
		return err
	}

	c.Process = &process{cmd: PrepareCommand(ctx, c.BinPath, args, nil, c.Dir)}

	logIfErr(c.Process.SetOutputFile(c.LogFile()))
	return c.Process.Start()
}

// Addr return addresses that can be connected by MySQL clients.
func (c *TiProxy) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(c.Host), c.Port)
}

// Component return component name.
func (c *TiProxy) Component() string {
	return "tiproxy"
}

// LogFile return the log file.
func (c *TiProxy) LogFile() string {
	return filepath.Join(c.Dir, "tiproxy.log")
}
