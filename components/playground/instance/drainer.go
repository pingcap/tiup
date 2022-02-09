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
)

// Drainer represent a drainer instance.
type Drainer struct {
	instance
	pds []*PDInstance
	Process
}

var _ Instance = &Drainer{}

// NewDrainer create a Drainer instance.
func NewDrainer(binPath string, dir, host, configPath string, id int, pds []*PDInstance) *Drainer {
	d := &Drainer{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 8250),
			ConfigPath: configPath,
		},
		pds: pds,
	}
	d.StatusPort = d.Port
	return d
}

// Component return component name.
func (d *Drainer) Component() string {
	return "drainer"
}

// LogFile return the log file name.
func (d *Drainer) LogFile() string {
	return filepath.Join(d.Dir, "drainer.log")
}

// Addr return the address of Drainer.
func (d *Drainer) Addr() string {
	return fmt.Sprintf("%s:%d", AdvertiseHost(d.Host), d.Port)
}

// NodeID return the node id of drainer.
func (d *Drainer) NodeID() string {
	return fmt.Sprintf("drainer_%d", d.ID)
}

// Start implements Instance interface.
func (d *Drainer) Start(ctx context.Context, version utils.Version) error {
	endpoints := pdEndpoints(d.pds, true)

	args := []string{
		fmt.Sprintf("--node-id=%s", d.NodeID()),
		fmt.Sprintf("--addr=%s:%d", d.Host, d.Port),
		fmt.Sprintf("--advertise-addr=%s:%d", AdvertiseHost(d.Host), d.Port),
		fmt.Sprintf("--pd-urls=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", d.LogFile()),
	}
	if d.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", d.ConfigPath))
	}

	var err error
	if d.BinPath, err = tiupexec.PrepareBinary("drainer", version, d.BinPath); err != nil {
		return err
	}
	d.Process = &process{cmd: PrepareCommand(ctx, d.BinPath, args, nil, d.Dir)}

	logIfErr(d.Process.SetOutputFile(d.LogFile()))
	return d.Process.Start()
}
