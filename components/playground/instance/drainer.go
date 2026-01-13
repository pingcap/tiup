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

	"github.com/pingcap/tiup/pkg/utils"
)

// Drainer represent a drainer instance.
type Drainer struct {
	instance
	pds []*PDInstance
}

var _ Instance = &Drainer{}

// NewDrainer create a Drainer instance.
func NewDrainer(shOpt SharedOptions, binPath string, dir, host, configPath string, id int, pds []*PDInstance) *Drainer {
	d := &Drainer{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 8250, shOpt.PortOffset),
			ConfigPath: configPath,
			role:       "drainer",
		},
		pds: pds,
	}
	d.StatusPort = d.Port
	return d
}

// LogFile return the log file name.
func (d *Drainer) LogFile() string {
	return filepath.Join(d.Dir, "drainer.log")
}

// Addr return the address of Drainer.
func (d *Drainer) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(d.Host), d.Port)
}

// Start implements Instance interface.
func (d *Drainer) Start(ctx context.Context) error {
	endpoints := pdEndpoints(d.pds, true)

	args := []string{
		fmt.Sprintf("--node-id=%s", d.Name()),
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(d.Host, d.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(d.Host), d.Port)),
		fmt.Sprintf("--pd-urls=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", d.LogFile()),
	}
	if d.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", d.ConfigPath))
	}

	return d.PrepareProcess(ctx, d.BinPath, args, nil, d.Dir)
}
