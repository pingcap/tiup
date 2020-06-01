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

package telemetry

import (
	"context"
	"runtime"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
)

// FillNodeInfo fill HardwareInfo and Os info.
func FillNodeInfo(ctx context.Context, info *NodeInfo) (err error) {
	info.Hardware, err = GetHardwareInfo(ctx)
	if err != nil {
		return
	}

	info.Os, err = GetOSInfo(ctx)
	if err != nil {
		return
	}

	return nil
}

// GetHardwareInfo get the HardwareInfo.
func GetHardwareInfo(ctx context.Context) (info HardwareInfo, err error) {
	if virt, role, err := host.VirtualizationWithContext(ctx); err == nil && role == "guest" {
		info.Virtualization = virt
	}

	if l, err := load.AvgWithContext(ctx); err == nil {
		info.Loadavg15 = float32(l.Load15)
	}

	// Fill cpu info
	info.Cpu.Numcpu = int32(runtime.NumCPU())
	if cpus, err := cpu.InfoWithContext(ctx); err == nil && len(cpus) > 0 {
		info.Cpu.Sockets = int32(len(cpus))
		c := cpus[0]
		info.Cpu.Cores = c.Cores
		info.Cpu.Model = c.ModelName
		info.Cpu.Mhz = float32(c.Mhz)
		info.Cpu.Features = c.Flags
	}
	// Fill mem info
	if m, err := mem.VirtualMemory(); err == nil {
		info.Mem.Available = m.Available
		info.Mem.Total = m.Total
	}

	return
}

// GetOSInfo get the OSInfo.
func GetOSInfo(ctx context.Context) (info OSInfo, err error) {
	platform, family, version, err := host.PlatformInformationWithContext(ctx)
	if err != nil {
		return
	}

	info.Platform = platform
	info.Family = family
	info.Version = version

	return
}
