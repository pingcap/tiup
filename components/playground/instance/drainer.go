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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

// Drainer represent a drainer instance.
type Drainer struct {
	instance
	pds []*PDInstance
	cmd *exec.Cmd
}

var _ Instance = &Drainer{}

// NewDrainer create a Drainer instance.
func NewDrainer(binPath string, dir, host, configPath string, id int, pds []*PDInstance) *Drainer {
	return &Drainer{
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
}

// Addr return the address of Drainer.
func (d *Drainer) Addr() string {
	return fmt.Sprintf("%s:%d", advertiseHost(d.Host), d.Port)
}

// NodeID return the node id of drainer.
func (d *Drainer) NodeID() string {
	return fmt.Sprintf("drainer_%d", d.ID)
}

// Start implements Instance interface.
func (d *Drainer) Start(ctx context.Context, version v0manifest.Version) error {
	if err := os.MkdirAll(d.Dir, 0755); err != nil {
		return err
	}

	var urls []string
	for _, pd := range d.pds {
		urls = append(urls, fmt.Sprintf("http://%s:%d", pd.Host, pd.StatusPort))
	}

	args := []string{
		"tiup", fmt.Sprintf("--binpath=%s", d.BinPath),
		CompVersion("drainer", version),
		fmt.Sprintf("--node-id=%s", d.NodeID()),
		fmt.Sprintf("--addr=%s:%d", d.Host, d.Port),
		fmt.Sprintf("--advertise-addr=%s:%d", advertiseHost(d.Host), d.Port),
		fmt.Sprintf("--pd-urls=%s", strings.Join(urls, ",")),
		fmt.Sprintf("--log-file=%s", filepath.Join(d.Dir, "drainer.log")),
	}
	if d.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", d.ConfigPath))
	}

	d.cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	d.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, d.Dir),
	)
	d.cmd.Stderr = os.Stderr
	d.cmd.Stdout = os.Stdout

	return d.cmd.Start()
}

// Wait implements Instance interface.
func (d *Drainer) Wait() error {
	return d.cmd.Wait()
}

// Pid implements Instance interface.
func (d *Drainer) Pid() int {
	return d.cmd.Process.Pid
}
