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

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceTiCDC is the service ID for TiCDC.
	ServiceTiCDC ServiceID = "ticdc"
	// ServiceTiKVCDC is the service ID for TiKV CDC.
	ServiceTiKVCDC ServiceID = "tikv-cdc"

	// ComponentCDC is the repository component ID for TiCDC.
	ComponentCDC RepoComponentID = "cdc"
	// ComponentTiKVCDC is the repository component ID for TiKV CDC.
	ComponentTiKVCDC RepoComponentID = "tikv-cdc"
)

// TiCDCPlan is the service-specific plan for TiCDC.
type TiCDCPlan struct{ PDAddrs []string }

// TiKVCDCPlan is the service-specific plan for TiKV-CDC.
type TiKVCDCPlan struct{ PDAddrs []string }

// TiCDC represent a ticdc instance.
type TiCDC struct {
	ProcessInfo
	Plan TiCDCPlan
}

var _ Process = &TiCDC{}

// TiKVCDCInstance represent a TiKV-CDC instance.
type TiKVCDCInstance struct {
	ProcessInfo
	Plan TiKVCDCPlan
}

var _ Process = &TiKVCDCInstance{}

func init() {
	RegisterComponentDisplayName(ComponentCDC, "TiCDC")
	RegisterServiceDisplayName(ServiceTiCDC, "TiCDC")
	RegisterComponentDisplayName(ComponentTiKVCDC, "TiKV-CDC")
	RegisterServiceDisplayName(ServiceTiKVCDC, "TiKV-CDC")

	registerPlannedProcessFactory(ServiceTiCDC, func(plan ServicePlan, info ProcessInfo, _ SharedOptions, _ string) (Process, error) {
		if plan.TiCDC == nil {
			name := info.Name()
			if name == "" {
				name = ServiceTiCDC.String()
			}
			return nil, errors.Errorf("missing ticdc plan for %s", name)
		}
		return &TiCDC{Plan: *plan.TiCDC, ProcessInfo: info}, nil
	})
	registerPlannedProcessFactory(ServiceTiKVCDC, func(plan ServicePlan, info ProcessInfo, _ SharedOptions, _ string) (Process, error) {
		if plan.TiKVCDC == nil {
			name := info.Name()
			if name == "" {
				name = ServiceTiKVCDC.String()
			}
			return nil, errors.Errorf("missing tikv-cdc plan for %s", name)
		}
		return &TiKVCDCInstance{Plan: *plan.TiKVCDC, ProcessInfo: info}, nil
	})
}

// Prepare builds the TiCDC process command.
func (c *TiCDC) Prepare(ctx context.Context) error {
	info := c.Info()
	endpoints := make([]string, 0, len(c.Plan.PDAddrs))
	for _, addr := range c.Plan.PDAddrs {
		if addr == "" {
			continue
		}
		endpoints = append(endpoints, "http://"+addr)
	}

	args := []string{
		"server",
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(c.Host, c.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(c.Host), c.Port)),
		fmt.Sprintf("--pd=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", c.LogFile()),
	}
	clusterVersion := string(c.Version)
	if tidbver.TiCDCSupportConfigFile(clusterVersion) {
		if c.ConfigPath != "" {
			args = append(args, fmt.Sprintf("--config=%s", c.ConfigPath))
		}
		if tidbver.TiCDCSupportDataDir(clusterVersion) {
			args = append(args, fmt.Sprintf("--data-dir=%s", filepath.Join(c.Dir, "data")))
		} else {
			args = append(args, fmt.Sprintf("--sort-dir=%s/tmp/sorter", filepath.Join(c.Dir, "data")))
		}
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, c.BinPath, args, nil, c.Dir)}
	return nil
}

// LogFile return the log file.
func (c *TiCDC) LogFile() string {
	return filepath.Join(c.Dir, "ticdc.log")
}

// Prepare builds the TiKV-CDC process command.
func (c *TiKVCDCInstance) Prepare(ctx context.Context) error {
	info := c.Info()
	endpoints := make([]string, 0, len(c.Plan.PDAddrs))
	for _, addr := range c.Plan.PDAddrs {
		if addr == "" {
			continue
		}
		endpoints = append(endpoints, "http://"+addr)
	}

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
