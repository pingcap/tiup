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

package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceDrainer is the service ID for Drainer.
	ServiceDrainer ServiceID = "drainer"

	// ComponentDrainer is the repository component ID for Drainer.
	ComponentDrainer RepoComponentID = "drainer"
)

// Drainer represent a drainer instance.
type Drainer struct {
	ProcessInfo
	PDs []*PDInstance
}

var _ Process = &Drainer{}

func init() {
	RegisterComponentDisplayName(ComponentDrainer, "Drainer")
	RegisterServiceDisplayName(ServiceDrainer, "Drainer")
}

// LogFile return the log file name.
func (d *Drainer) LogFile() string {
	return filepath.Join(d.Dir, "drainer.log")
}

// Addr return the address of Drainer.
func (d *Drainer) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(d.Host), d.Port)
}

// Prepare builds the Drainer process command.
func (d *Drainer) Prepare(ctx context.Context) error {
	info := d.Info()
	endpoints := pdEndpoints(d.PDs, true)

	args := []string{
		fmt.Sprintf("--node-id=%s", info.Name()),
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(d.Host, d.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(d.Host), d.Port)),
		fmt.Sprintf("--pd-urls=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", d.LogFile()),
	}
	if d.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", d.ConfigPath))
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, d.BinPath, args, nil, d.Dir)}
	return nil
}
