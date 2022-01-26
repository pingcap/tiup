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

package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/AstroProfundis/sysinfo"
	"github.com/pingcap/tidb-insight/collector/insight"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"go.uber.org/zap"
)

// CheckOptions control the list of checks to be performed
type CheckOptions struct {
	// checks that are disabled by default
	EnableCPU  bool
	EnableMem  bool
	EnableDisk bool

	// pre-defined goups of checks
	// GroupMinimal bool // a minimal set of checks
}

// Names of checks
var (
	CheckNameGeneral       = "general" // errors that don't fit any specific check
	CheckNameNTP           = "ntp"
	CheckNameChrony        = "chrony"
	CheckNameOSVer         = "os-version"
	CheckNameSwap          = "swap"
	CheckNameSysctl        = "sysctl"
	CheckNameCPUThreads    = "cpu-cores"
	CheckNameCPUGovernor   = "cpu-governor"
	CheckNameDisks         = "disk"
	CheckNamePortListen    = "listening-port"
	CheckNameEpoll         = "epoll-exclusive"
	CheckNameMem           = "memory"
	CheckNameNet           = "network"
	CheckNameLimits        = "limits"
	CheckNameSysService    = "service"
	CheckNameSELinux       = "selinux"
	CheckNameCommand       = "command"
	CheckNameFio           = "fio"
	CheckNameTHP           = "thp"
	CheckNameDirPermission = "permission"
	CheckNameDirExist      = "exist"
)

// CheckResult is the result of a check
type CheckResult struct {
	Name string // Name of the check
	Err  error  // An embedded error
	Warn bool   // The check didn't pass, but not a big problem
	Msg  string // A message or description
}

// Error implements the error interface
func (c CheckResult) Error() string {
	return c.Err.Error()
}

// String returns a readable string of the error
func (c CheckResult) String() string {
	return fmt.Sprintf("check failed for %s: %s", c.Name, c.Err)
}

// Unwrap implements the Wrapper interface
func (c CheckResult) Unwrap() error {
	return c.Err
}

// IsWarning checks if the result is a warning error
func (c CheckResult) IsWarning() bool {
	return c.Warn
}

// Passed checks if the result is a success
func (c CheckResult) Passed() bool {
	return c.Err == nil
}

// CheckSystemInfo performs checks with basic system info
func CheckSystemInfo(opt *CheckOptions, rawData []byte) []*CheckResult {
	var results []*CheckResult
	var insightInfo insight.InsightInfo
	if err := json.Unmarshal(rawData, &insightInfo); err != nil {
		return append(results, &CheckResult{
			Name: CheckNameGeneral,
			Err:  err,
		})
	}

	// check basic system info
	results = append(results, checkSysInfo(opt, &insightInfo.SysInfo)...)

	// check time sync status
	switch {
	case insightInfo.ChronyStat.LeapStatus != "none":
		results = append(results, checkChrony(&insightInfo.ChronyStat))
	case insightInfo.NTP.Status != "none":
		results = append(results, checkNTP(&insightInfo.NTP))
	default:
		results = append(results,
			&CheckResult{
				Name: CheckNameNTP,
				Err:  fmt.Errorf("The NTPd daemon or Chronyd daemon may be not installed"),
				Warn: true,
			},
		)
	}

	epollResult := &CheckResult{
		Name: CheckNameEpoll,
	}
	if !insightInfo.EpollExcl {
		epollResult.Err = fmt.Errorf("epoll exclusive is not supported")
	}
	results = append(results, epollResult)

	return results
}

func checkSysInfo(opt *CheckOptions, sysInfo *sysinfo.SysInfo) []*CheckResult {
	var results []*CheckResult

	results = append(results, checkOSInfo(opt, &sysInfo.OS))

	// check cpu capacities
	results = append(results, checkCPU(opt, &sysInfo.CPU)...)

	// check memory size
	results = append(results, checkMem(opt, &sysInfo.Memory)...)

	// check network
	results = append(results, checkNetwork(opt, sysInfo.Network)...)

	return results
}

func checkOSInfo(opt *CheckOptions, osInfo *sysinfo.OS) *CheckResult {
	result := &CheckResult{
		Name: CheckNameOSVer,
		Msg:  fmt.Sprintf("OS is %s %s", osInfo.Name, osInfo.Release),
	}

	// check OS vendor
	switch osInfo.Vendor {
	case "amzn":
		// Amazon Linux 2 is based on CentOS 7 and is recommended for
		// AWS Graviton 2 (ARM64) deployments.
		if ver, _ := strconv.ParseFloat(osInfo.Version, 64); ver < 2 || ver >= 3 {
			result.Err = fmt.Errorf("%s %s not supported, use version 2 please",
				osInfo.Name, osInfo.Release)
			return result
		}
	case "centos", "redhat", "rhel":
		// check version
		// CentOS 8 is known to be not working, and we don't have plan to support it
		// as of now, we may add support for RHEL 8 based systems in the future.
		if ver, _ := strconv.ParseFloat(osInfo.Version, 64); ver < 7 || ver >= 8 {
			result.Err = fmt.Errorf("%s %s not supported, use version 7 please",
				osInfo.Name, osInfo.Release)
			return result
		}
	case "debian":
		// debian support is not fully tested, but we suppose it should work
		msg := "debian support is not fully tested, be careful"
		result.Err = fmt.Errorf("%s (%s)", result.Msg, msg)
		result.Warn = true
		if ver, _ := strconv.ParseFloat(osInfo.Version, 64); ver < 9 {
			result.Err = fmt.Errorf("%s %s not supported, use version 9 or higher (%s)",
				osInfo.Name, osInfo.Release, msg)
			result.Warn = false
			return result
		}
	case "ubuntu":
		// ubuntu support is not fully tested, but we suppose it should work
		msg := "ubuntu support is not fully tested, be careful"
		result.Err = fmt.Errorf("%s (%s)", result.Msg, msg)
		result.Warn = true
		if ver, _ := strconv.ParseFloat(osInfo.Version, 64); ver < 18.04 {
			result.Err = fmt.Errorf("%s %s not supported, use version 18.04 or higher (%s)",
				osInfo.Name, osInfo.Release, msg)
			result.Warn = false
			return result
		}
	default:
		result.Err = fmt.Errorf("os vendor %s not supported", osInfo.Vendor)
		return result
	}

	// TODO: check OS architecture

	return result
}

func checkNTP(ntpInfo *insight.TimeStat) *CheckResult {
	result := &CheckResult{
		Name: CheckNameNTP,
	}

	if ntpInfo.Status == "none" {
		zap.L().Info("The NTPd daemon may be not installed, skip.")
		return result
	}

	if ntpInfo.Sync == "none" {
		result.Err = fmt.Errorf("The NTPd daemon may be not start")
		result.Warn = true
		return result
	}

	// check if time offset greater than +- 500ms
	if math.Abs(ntpInfo.Offset) >= 500 {
		result.Err = fmt.Errorf("time offset %fms too high", ntpInfo.Offset)
	}
	return result
}

func checkChrony(chronyInfo *insight.ChronyStat) *CheckResult {
	result := &CheckResult{
		Name: CheckNameChrony,
	}

	if chronyInfo.LeapStatus == "none" {
		zap.L().Info("The Chrony daemon may be not installed, skip.")
		return result
	}

	// check if time offset greater than +- 500ms
	if math.Abs(chronyInfo.LastOffset) >= 500 {
		result.Err = fmt.Errorf("time offset %fms too high", chronyInfo.LastOffset)
	}
	return result
}

func checkCPU(opt *CheckOptions, cpuInfo *sysinfo.CPU) []*CheckResult {
	var results []*CheckResult
	if opt.EnableCPU && cpuInfo.Threads < 16 {
		results = append(results, &CheckResult{
			Name: CheckNameCPUThreads,
			Err:  fmt.Errorf("CPU thread count %d too low, needs 16 or more", cpuInfo.Threads),
		})
	} else {
		results = append(results, &CheckResult{
			Name: CheckNameCPUThreads,
			Msg:  fmt.Sprintf("number of CPU cores / threads: %d", cpuInfo.Threads),
		})
	}

	// check for CPU frequency governor
	if cpuInfo.Governor != "" {
		if cpuInfo.Governor != "performance" {
			results = append(results, &CheckResult{
				Name: CheckNameCPUGovernor,
				Err:  fmt.Errorf("CPU frequency governor is %s, should use performance", cpuInfo.Governor),
			})
		} else {
			results = append(results, &CheckResult{
				Name: CheckNameCPUGovernor,
				Msg:  fmt.Sprintf("CPU frequency governor is %s", cpuInfo.Governor),
			})
		}
	} else {
		results = append(results, &CheckResult{
			Name: CheckNameCPUGovernor,
			Err:  fmt.Errorf("Unable to determine current CPU frequency governor policy"),
			Warn: true,
		})
	}

	return results
}

func checkMem(opt *CheckOptions, memInfo *sysinfo.Memory) []*CheckResult {
	var results []*CheckResult
	if memInfo.Swap > 0 {
		results = append(results, &CheckResult{
			Name: CheckNameSwap,
			Err:  fmt.Errorf("swap is enabled, please disable it for best performance"),
		})
	}

	// 32GB
	if opt.EnableMem && memInfo.Size < 1024*32 {
		results = append(results, &CheckResult{
			Name: CheckNameMem,
			Err:  fmt.Errorf("memory size %dMB too low, needs 32GB or more", memInfo.Size),
		})
	} else {
		results = append(results, &CheckResult{
			Name: CheckNameMem,
			Msg:  fmt.Sprintf("memory size is %dMB", memInfo.Size),
		})
	}

	return results
}

func checkNetwork(opt *CheckOptions, networkDevices []sysinfo.NetworkDevice) []*CheckResult {
	var results []*CheckResult
	for _, netdev := range networkDevices {
		// ignore the network devices that cannot be detected
		if netdev.Speed == 0 {
			continue
		}
		if netdev.Speed >= 1000 {
			results = append(results, &CheckResult{
				Name: CheckNameNet,
				Msg:  fmt.Sprintf("network speed of %s is %dMB", netdev.Name, netdev.Speed),
			})
		} else {
			results = append(results, &CheckResult{
				Name: CheckNameNet,
				Err:  fmt.Errorf("network speed of %s is %dMB too low, needs 1GB or more", netdev.Name, netdev.Speed),
			})
		}
	}

	return results
}

// CheckSysLimits checks limits in /etc/security/limits.conf
func CheckSysLimits(opt *CheckOptions, user string, l []byte) []*CheckResult {
	var results []*CheckResult

	var (
		stackSoft  int
		nofileSoft int
		nofileHard int
	)

	for _, line := range strings.Split(string(l), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != user {
			continue
		}

		switch fields[2] {
		case "nofile":
			if fields[1] == "soft" {
				nofileSoft, _ = strconv.Atoi(fields[3])
			} else {
				nofileHard, _ = strconv.Atoi(fields[3])
			}
		case "stack":
			if fields[1] == "soft" {
				stackSoft, _ = strconv.Atoi(fields[3])
			}
		}
	}

	if nofileSoft < 1000000 {
		results = append(results, &CheckResult{
			Name: CheckNameLimits,
			Err:  fmt.Errorf("soft limit of 'nofile' for user '%s' is not set or too low", user),
			Msg:  fmt.Sprintf("%s    soft    nofile    1000000", user),
		})
	}
	if nofileHard < 1000000 {
		results = append(results, &CheckResult{
			Name: CheckNameLimits,
			Err:  fmt.Errorf("hard limit of 'nofile' for user '%s' is not set or too low", user),
			Msg:  fmt.Sprintf("%s    hard    nofile    1000000", user),
		})
	}
	if stackSoft < 10240 {
		results = append(results, &CheckResult{
			Name: CheckNameLimits,
			Err:  fmt.Errorf("soft limit of 'stack' for user '%s' is not set or too low", user),
			Msg:  fmt.Sprintf("%s    soft    stack    10240", user),
		})
	}

	// all pass
	if len(results) < 1 {
		results = append(results, &CheckResult{
			Name: CheckNameLimits,
		})
	}

	return results
}

// CheckKernelParameters checks kernel parameter values
func CheckKernelParameters(opt *CheckOptions, p []byte) []*CheckResult {
	var results []*CheckResult

	for _, line := range strings.Split(string(p), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		switch fields[0] {
		case "fs.file-max":
			val, _ := strconv.Atoi(fields[2])
			if val < 1000000 {
				results = append(results, &CheckResult{
					Name: CheckNameSysctl,
					Err:  fmt.Errorf("fs.file-max = %d, should be greater than 1000000", val),
					Msg:  "fs.file-max = 1000000",
				})
			}
		case "net.core.somaxconn":
			val, _ := strconv.Atoi(fields[2])
			if val < 32768 {
				results = append(results, &CheckResult{
					Name: CheckNameSysctl,
					Err:  fmt.Errorf("net.core.somaxconn = %d, should be greater than 32768", val),
					Msg:  "net.core.somaxconn = 32768",
				})
			}
		case "net.ipv4.tcp_tw_recycle":
			val, _ := strconv.Atoi(fields[2])
			if val != 0 {
				results = append(results, &CheckResult{
					Name: CheckNameSysctl,
					Err:  fmt.Errorf("net.ipv4.tcp_tw_recycle = %d, should be 0", val),
					Msg:  "net.ipv4.tcp_tw_recycle = 0",
				})
			}
		case "net.ipv4.tcp_syncookies":
			val, _ := strconv.Atoi(fields[2])
			if val != 0 {
				results = append(results, &CheckResult{
					Name: CheckNameSysctl,
					Err:  fmt.Errorf("net.ipv4.tcp_syncookies = %d, should be 0", val),
					Msg:  "net.ipv4.tcp_syncookies = 0",
				})
			}
		case "vm.overcommit_memory":
			val, _ := strconv.Atoi(fields[2])
			if opt.EnableMem && val != 0 && val != 1 {
				results = append(results, &CheckResult{
					Name: CheckNameSysctl,
					Err:  fmt.Errorf("vm.overcommit_memory = %d, should be 0 or 1", val),
					Msg:  "vm.overcommit_memory = 1",
				})
			}
		case "vm.swappiness":
			val, _ := strconv.Atoi(fields[2])
			if val != 0 {
				results = append(results, &CheckResult{
					Name: CheckNameSysctl,
					Err:  fmt.Errorf("vm.swappiness = %d, should be 0", val),
					Msg:  "vm.swappiness = 0",
				})
			}
		}
	}

	// all pass
	if len(results) < 1 {
		results = append(results, &CheckResult{
			Name: CheckNameSysctl,
		})
	}

	return results
}

// CheckServices checks if a service is running on the host
func CheckServices(ctx context.Context, e ctxt.Executor, host, service string, disable bool) *CheckResult {
	result := &CheckResult{
		Name: CheckNameSysService,
	}

	// check if the service exist before checking its status, ignore when non-exist
	stdout, _, err := e.Execute(
		ctx,
		fmt.Sprintf(
			"systemctl list-unit-files --type service | grep -i %s.service | wc -l", service),
		true)
	if err != nil {
		result.Err = err
		return result
	}
	if cnt, _ := strconv.Atoi(strings.Trim(string(stdout), "\n")); cnt == 0 {
		if !disable {
			result.Err = fmt.Errorf("service %s not found, should be installed and started", service)
		}
		result.Msg = fmt.Sprintf("service %s not found, ignore", service)
		return result
	}

	active, err := GetServiceStatus(ctx, e, service+".service")
	if err != nil {
		result.Err = err
	}

	switch disable {
	case false:
		if !strings.Contains(active, "running") {
			result.Err = fmt.Errorf("service %s is not running", service)
			result.Msg = fmt.Sprintf("start %s.service", service)
		}
	case true:
		if strings.Contains(active, "running") {
			result.Err = fmt.Errorf("service %s is running but should be stopped", service)
			result.Msg = fmt.Sprintf("stop %s.service", service)
		}
	}

	return result
}

// CheckSELinux checks if SELinux is enabled on the host
func CheckSELinux(ctx context.Context, e ctxt.Executor) *CheckResult {
	result := &CheckResult{
		Name: CheckNameSELinux,
	}
	m := module.NewShellModule(module.ShellModuleConfig{
		// ignore grep errors, the file may not exist for some systems
		Command: "grep -E '^\\s*SELINUX=enforcing' /etc/selinux/config 2>/dev/null | wc -l",
		Sudo:    true,
	})
	stdout, stderr, err := m.Execute(ctx, e)
	if err != nil {
		result.Err = fmt.Errorf("%w %s", err, stderr)
		return result
	}
	out := strings.Trim(string(stdout), "\n")
	lines, err := strconv.Atoi(out)
	if err != nil {
		result.Err = fmt.Errorf("can not check SELinux status, please validate manually, %s", err)
		result.Warn = true
		return result
	}

	if lines > 0 {
		result.Err = fmt.Errorf("SELinux is not disabled")
	} else {
		result.Msg = "SELinux is disabled"
	}
	return result
}

// CheckListeningPort checks if the ports are already binded by some process on host
func CheckListeningPort(opt *CheckOptions, host string, topo *spec.Specification, rawData []byte) []*CheckResult {
	var results []*CheckResult
	ports := make(map[int]struct{})

	topo.IterInstance(func(inst spec.Instance) {
		if inst.GetHost() != host {
			return
		}
		for _, up := range inst.UsedPorts() {
			if _, found := ports[up]; !found {
				ports[up] = struct{}{}
			}
		}
	})

	for p := range ports {
		for _, line := range strings.Split(string(rawData), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 5 || fields[0] != "LISTEN" {
				continue
			}
			addr := strings.Split(fields[3], ":")
			lp, _ := strconv.Atoi(addr[len(addr)-1])
			if p == lp {
				results = append(results, &CheckResult{
					Name: CheckNamePortListen,
					Err:  fmt.Errorf("port %d is already in use", lp),
				})
				break // ss may report multiple entries for the same port
			}
		}
	}
	return results
}

// CheckPartitions checks partition info of data directories
func CheckPartitions(opt *CheckOptions, host string, topo *spec.Specification, rawData []byte) []*CheckResult {
	var results []*CheckResult
	var insightInfo insight.InsightInfo
	if err := json.Unmarshal(rawData, &insightInfo); err != nil {
		return append(results, &CheckResult{
			Name: CheckNameDisks,
			Err:  err,
		})
	}

	flt := flatPartitions(insightInfo.Partitions)
	parts := sortPartitions(flt)

	// check if multiple instances are using the same partition as data storeage
	type storePartitionInfo struct {
		comp string
		path string
	}
	uniqueStores := make(map[string][]storePartitionInfo) // host+partition -> info

	topo.IterInstance(func(inst spec.Instance) {
		if inst.GetHost() != host {
			return
		}
		for _, dataDir := range spec.MultiDirAbs(topo.GlobalOptions.User, inst.DataDir()) {
			if dataDir == "" {
				continue
			}

			blk := getDisk(parts, dataDir)
			if blk == nil {
				return
			}

			// only check for TiKV and TiFlash, other components are not that I/O sensitive
			switch inst.ComponentName() {
			case spec.ComponentTiKV,
				spec.ComponentTiFlash:
				usKey := fmt.Sprintf("%s:%s", host, blk.Mount.MountPoint)
				uniqueStores[usKey] = append(uniqueStores[usKey], storePartitionInfo{
					comp: inst.ComponentName(),
					path: dataDir,
				})
			}

			switch blk.Mount.FSType {
			case "ext4":
				if !strings.Contains(blk.Mount.Options, "nodelalloc") {
					results = append(results, &CheckResult{
						Name: CheckNameDisks,
						Err:  fmt.Errorf("mount point %s does not have 'nodelalloc' option set", blk.Mount.MountPoint),
					})
				}
				fallthrough
			case "xfs":
				if !strings.Contains(blk.Mount.Options, "noatime") {
					results = append(results, &CheckResult{
						Name: CheckNameDisks,
						Err:  fmt.Errorf("mount point %s does not have 'noatime' option set", blk.Mount.MountPoint),
						Warn: true,
					})
				}
			default:
				results = append(results, &CheckResult{
					Name: CheckNameDisks,
					Err: fmt.Errorf("mount point %s has an unsupported filesystem '%s'",
						blk.Mount.MountPoint, blk.Mount.FSType),
				})
			}
		}
	})

	for key, parts := range uniqueStores {
		if len(parts) > 1 {
			pathList := make([]string, 0)
			for _, p := range parts {
				pathList = append(pathList,
					fmt.Sprintf("%s:%s", p.comp, p.path),
				)
			}
			results = append(results, &CheckResult{
				Name: CheckNameDisks,
				Err: fmt.Errorf(
					"multiple components %s are using the same partition %s as data dir",
					strings.Join(pathList, ","),
					key,
				),
			})
		}
	}

	return results
}

func flatPartitions(parts []insight.BlockDev) []insight.BlockDev {
	var flatBlk []insight.BlockDev
	for _, blk := range parts {
		if len(blk.SubDev) > 0 {
			flatBlk = append(flatBlk, flatPartitions(blk.SubDev)...)
		}
		// blocks with empty mount points are ignored
		if blk.Mount.MountPoint != "" {
			flatBlk = append(flatBlk, blk)
		}
	}
	return flatBlk
}

func sortPartitions(parts []insight.BlockDev) []insight.BlockDev {
	// The longest mount point is at top of the list
	sort.Slice(parts, func(i, j int) bool {
		return len(parts[i].Mount.MountPoint) > len(parts[j].Mount.MountPoint)
	})

	return parts
}

// getDisk find the first block dev from the list that matches the given path
func getDisk(parts []insight.BlockDev, fullpath string) *insight.BlockDev {
	for _, blk := range parts {
		if strings.HasPrefix(fullpath, blk.Mount.MountPoint) {
			return &blk
		}
	}
	return nil
}

// CheckFIOResult parses and checks the result of fio test
func CheckFIOResult(rr, rw, lat []byte) []*CheckResult {
	var results []*CheckResult

	// check results for rand read test
	var rrRes map[string]interface{}
	if err := json.Unmarshal(rr, &rrRes); err != nil {
		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Err:  fmt.Errorf("error parsing result of random read test, %s", err),
		})
	} else if jobs, ok := rrRes["jobs"]; ok {
		readRes := jobs.([]interface{})[0].(map[string]interface{})["read"]
		readIOPS := readRes.(map[string]interface{})["iops"]

		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Msg:  fmt.Sprintf("IOPS of random read: %f", readIOPS.(float64)),
		})
	} else {
		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Err:  fmt.Errorf("error parsing result of random read test"),
		})
	}

	// check results for rand read write
	var rwRes map[string]interface{}
	if err := json.Unmarshal(rw, &rwRes); err != nil {
		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Err:  fmt.Errorf("error parsing result of random read write test, %s", err),
		})
	} else if jobs, ok := rwRes["jobs"]; ok {
		readRes := jobs.([]interface{})[0].(map[string]interface{})["read"]
		readIOPS := readRes.(map[string]interface{})["iops"]

		writeRes := jobs.([]interface{})[0].(map[string]interface{})["write"]
		writeIOPS := writeRes.(map[string]interface{})["iops"]

		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Msg:  fmt.Sprintf("IOPS of random read: %f, write: %f", readIOPS.(float64), writeIOPS.(float64)),
		})
	} else {
		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Err:  fmt.Errorf("error parsing result of random read write test"),
		})
	}

	// check results for read write latency
	var latRes map[string]interface{}
	if err := json.Unmarshal(lat, &latRes); err != nil {
		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Err:  fmt.Errorf("error parsing result of read write latency test, %s", err),
		})
	} else if jobs, ok := latRes["jobs"]; ok {
		readRes := jobs.([]interface{})[0].(map[string]interface{})["read"]
		readLat := readRes.(map[string]interface{})["lat_ns"]
		readLatAvg := readLat.(map[string]interface{})["mean"]

		writeRes := jobs.([]interface{})[0].(map[string]interface{})["write"]
		writeLat := writeRes.(map[string]interface{})["lat_ns"]
		writeLatAvg := writeLat.(map[string]interface{})["mean"]

		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Msg:  fmt.Sprintf("Latency of random read: %fns, write: %fns", readLatAvg.(float64), writeLatAvg.(float64)),
		})
	} else {
		results = append(results, &CheckResult{
			Name: CheckNameFio,
			Err:  fmt.Errorf("error parsing result of read write latency test"),
		})
	}

	return results
}

// CheckTHP checks THP in /sys/kernel/mm/transparent_hugepage/{enabled,defrag}
func CheckTHP(ctx context.Context, e ctxt.Executor) *CheckResult {
	result := &CheckResult{
		Name: CheckNameTHP,
	}

	m := module.NewShellModule(module.ShellModuleConfig{
		Command: fmt.Sprintf(`if [ -d %[1]s ]; then cat %[1]s/{enabled,defrag}; fi`, "/sys/kernel/mm/transparent_hugepage"),
		Sudo:    true,
	})
	stdout, stderr, err := m.Execute(ctx, e)
	if err != nil {
		result.Err = fmt.Errorf("%w %s", err, stderr)
		return result
	}

	for _, line := range strings.Split(strings.Trim(string(stdout), "\n"), "\n") {
		if len(line) > 0 && !strings.Contains(line, "[never]") {
			result.Err = fmt.Errorf("THP is enabled, please disable it for best performance")
			return result
		}
	}

	result.Msg = "THP is disabled"
	return result
}

// CheckJRE checks if java command is available for TiSpark nodes
func CheckJRE(ctx context.Context, e ctxt.Executor, host string, topo *spec.Specification) []*CheckResult {
	var results []*CheckResult

	topo.IterInstance(func(inst spec.Instance) {
		if inst.ComponentName() != spec.ComponentTiSpark {
			return
		}

		// check if java cli is available
		stdout, stderr, err := e.Execute(ctx, "java -version", false)
		if err != nil {
			results = append(results, &CheckResult{
				Name: CheckNameCommand,
				Err:  fmt.Errorf("java not usable, %s", strings.Trim(string(stderr), "\n")),
				Msg:  "JRE is not installed properly or not set in PATH",
			})
			return
		}
		if len(stderr) > 0 {
			// java -version returns as below:
			// openjdk version "1.8.0_265"
			// openjdk version "11.0.8" 2020-07-14
			line := strings.Split(string(stderr), "\n")[0]
			fields := strings.Split(line, `"`)
			ver := strings.TrimSpace(fields[1])
			if strings.Compare(ver, "1.8") < 0 {
				results = append(results, &CheckResult{
					Name: CheckNameCommand,
					Err:  fmt.Errorf("java version %s is not supported, use Java 8 (1.8)+", ver),
					Msg:  "Installed JRE is not Java 8+",
				})
			} else {
				results = append(results, &CheckResult{
					Name: CheckNameCommand,
					Msg:  "java: " + strings.Split(string(stderr), "\n")[0],
				})
			}
		} else {
			results = append(results, &CheckResult{
				Name: CheckNameCommand,
				Err:  fmt.Errorf("unknown output of java %s", stdout),
				Msg:  "java: " + strings.Split(string(stdout), "\n")[0],
				Warn: true,
			})
		}
	})

	return results
}

// CheckDirPermission checks if the user can write to given path
func CheckDirPermission(ctx context.Context, e ctxt.Executor, user, path string) []*CheckResult {
	var results []*CheckResult

	_, stderr, err := e.Execute(ctx,
		fmt.Sprintf(
			"/usr/bin/sudo -u %[1]s touch %[2]s/.tiup_cluster_check_file && rm -f %[2]s/.tiup_cluster_check_file",
			user,
			path,
		),
		false)
	if err != nil || len(stderr) > 0 {
		results = append(results, &CheckResult{
			Name: CheckNameDirPermission,
			Err:  fmt.Errorf("unable to write to dir %s: %s", path, strings.Split(string(stderr), "\n")[0]),
			Msg:  fmt.Sprintf("%s: %s", path, err),
		})
	} else {
		results = append(results, &CheckResult{
			Name: CheckNameDirPermission,
			Msg:  fmt.Sprintf("%s is writable", path),
		})
	}

	return results
}

// CheckDirIsExist check if the directory exists
func CheckDirIsExist(ctx context.Context, e ctxt.Executor, path string) []*CheckResult {
	var results []*CheckResult

	if path == "" {
		return results
	}

	req, _, _ := e.Execute(ctx,
		fmt.Sprintf(
			"[ -e %s ] && echo 1",
			path,
		),
		false)

	if strings.ReplaceAll(string(req), "\n", "") == "1" {
		results = append(results, &CheckResult{
			Name: CheckNameDirExist,
			Err:  fmt.Errorf("%s already exists", path),
			Msg:  fmt.Sprintf("%s already exists", path),
		})
	}

	return results
}
