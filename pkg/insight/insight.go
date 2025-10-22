// Copyright 2018 PingCAP, Inc.
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

package insight

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/AstroProfundis/sysinfo"
	"github.com/pingcap/tiup/pkg/kmsg"
)

// Meta are information about insight itself
type Meta struct {
	Timestamp time.Time `json:"timestamp"`
	UPTime    float64   `json:"uptime,omitempty"`
	IdleTime  float64   `json:"idle_time,omitempty"`
	SiVer     string    `json:"sysinfo_ver"`
	GitBranch string    `json:"git_branch"`
	GitCommit string    `json:"git_commit"`
	GoVersion string    `json:"go_version"`
}

// Info are information gathered from the system
type Info struct {
	Meta       Meta            `json:"meta"`
	SysInfo    sysinfo.SysInfo `json:"sysinfo"`
	NTP        TimeStat        `json:"ntp"`
	ChronyStat ChronyStat      `json:"chrony"`
	Partitions []BlockDev      `json:"partitions,omitempty"`
	ProcStats  []ProcessStat   `json:"proc_stats,omitempty"`
	EpollExcl  bool            `json:"epoll_exclusive,omitempty"`
	SysConfig  *SysCfg         `json:"system_configs,omitempty"`
	DMesg      []*kmsg.Msg     `json:"dmesg,omitempty"`
	Sockets    []Socket        `json:"sockets,omitempty"`
}

// Options sets options for info collection
type Options struct {
	Pid    string
	Proc   bool
	Syscfg bool // collect kernel configs or not
	Dmesg  bool // collect kernel logs or not
}

// GetInfo collects Info
//
//revive:disable:get-return
func (info *Info) GetInfo(opts Options) {
	var pidList []string
	if len(opts.Pid) > 0 {
		pidList = strings.Split(opts.Pid, ",")
	}

	info.Meta.getMeta(pidList)
	if opts.Proc {
		info.ProcStats = GetProcessStats(pidList)
		return
	}

	info.SysInfo.GetSysInfo()
	info.NTP.getNTPInfo()
	info.ChronyStat.getChronyInfo()
	info.Partitions = GetPartitionStats()
	switch runtime.GOOS {
	case "android",
		"darwin",
		"dragonfly",
		"freebsd",
		"linux",
		"netbsd",
		"openbsd":
		info.EpollExcl = checkEpollExclusive()
	default:
		info.EpollExcl = false
	}

	if opts.Syscfg {
		info.SysConfig = &SysCfg{}
		info.SysConfig.getSysCfg()
	}
	if opts.Dmesg {
		_ = info.collectDmsg()
	}

	_ = info.collectSockets()
}

func (meta *Meta) getMeta(pidList []string) {
	meta.Timestamp = time.Now()
	if sysUptime, sysIdleTime, err := GetSysUptime(); err == nil {
		meta.UPTime = sysUptime
		meta.IdleTime = sysIdleTime
	}

	meta.SiVer = sysinfo.Version
	meta.GitBranch = GitBranch
	meta.GitCommit = GitCommit
	meta.GoVersion = fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

//revive:enable:get-return
