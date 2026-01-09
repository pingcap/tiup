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

package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceTiKVCDC is the service ID for TiKV CDC.
	ServiceTiKVCDC ServiceID = "tikv-cdc"

	// ComponentTiKVCDC is the repository component ID for TiKV CDC.
	ComponentTiKVCDC RepoComponentID = "tikv-cdc"
)

// TiKVCDCInstance represent a TiKV-CDC instance.
type TiKVCDCInstance struct {
	ProcessInfo
	PDs []*PDInstance
}

var _ Process = &TiKVCDCInstance{}

func init() {
	RegisterComponentDisplayName(ComponentTiKVCDC, "TiKV-CDC")
	RegisterServiceDisplayName(ServiceTiKVCDC, "TiKV-CDC")
}

// Prepare builds the TiKV-CDC process command.
func (c *TiKVCDCInstance) Prepare(ctx context.Context) error {
	info := c.Info()
	endpoints := pdEndpoints(c.PDs, true)

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

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, c.BinPath, args, nil, c.Dir)}
	return nil
}

// LogFile return the log file.
func (c *TiKVCDCInstance) LogFile() string {
	return filepath.Join(c.Dir, "tikv_cdc.log")
}
