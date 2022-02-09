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
	"net/http"
	"path/filepath"
	"strings"
	"time"

	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

// Pump represent a pump instance.
type Pump struct {
	instance
	pds []*PDInstance
	Process
}

var _ Instance = &Pump{}

// NewPump create a Pump instance.
func NewPump(binPath string, dir, host, configPath string, id int, pds []*PDInstance) *Pump {
	pump := &Pump{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 8249),
			ConfigPath: configPath,
		},
		pds: pds,
	}
	pump.StatusPort = pump.Port
	return pump
}

// NodeID return the node id of pump.
func (p *Pump) NodeID() string {
	return fmt.Sprintf("pump_%d", p.ID)
}

// Ready return nil when pump is ready to serve.
func (p *Pump) Ready(ctx context.Context) error {
	url := fmt.Sprintf("http://%s:%d/status", p.Host, p.Port)

	ready := func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 200
	}

	for {
		if ready() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			// just retry
		}
	}
}

// Addr return the address of Pump.
func (p *Pump) Addr() string {
	return fmt.Sprintf("%s:%d", AdvertiseHost(p.Host), p.Port)
}

// Start implements Instance interface.
func (p *Pump) Start(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(p.pds, true)

	args := []string{
		fmt.Sprintf("--node-id=%s", p.NodeID()),
		fmt.Sprintf("--addr=%s:%d", p.Host, p.Port),
		fmt.Sprintf("--advertise-addr=%s:%d", AdvertiseHost(p.Host), p.Port),
		fmt.Sprintf("--pd-urls=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", p.LogFile()),
	}
	if p.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", p.ConfigPath))
	}

	var err error
	if p.BinPath, err = tiupexec.PrepareBinary("pump", version, p.BinPath); err != nil {
		return err
	}
	p.Process = &process{cmd: PrepareCommand(ctx, p.BinPath, args, nil, p.Dir)}

	logIfErr(p.Process.SetOutputFile(p.LogFile()))
	return p.Process.Start()
}

// Component return component name.
func (p *Pump) Component() string {
	return "pump"
}

// LogFile return the log file.
func (p *Pump) LogFile() string {
	return filepath.Join(p.Dir, "pump.log")
}
