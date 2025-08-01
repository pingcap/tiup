// process
package insight

import (
	"log"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"
)

// ProcessStat contains basic info of a running process
type ProcessStat struct {
	Name      string                  `json:"name"`
	Pid       int32                   `json:"pid"`
	Exec      string                  `json:"exec"`
	Cmdline   string                  `json:"cmd"`
	Status    string                  `json:"status"`
	StartTime float64                 `json:"start_time"`
	CPUTimes  *cpu.TimesStat          `json:"cpu_times"`
	Memory    *process.MemoryInfoStat `json:"memory"`
	Rlimit    []RlimitUsage           `json:"resource_limit"`
}

// RlimitUsage is the resource limit usage of a process
type RlimitUsage struct {
	Resource string `json:"resource"`
	Soft     int64  `json:"soft"`
	Hard     int64  `json:"hard"`
	Used     uint64 `json:"used"`
}

// GetProcessStats gets processlist statistics
func GetProcessStats(pidList []string) []ProcessStat {
	if pidList != nil {
		return getProcStatsByPIDList(pidList)
	}
	return getProcStatsByName()
}

func getProcStatsByPIDList(pidList []string) []ProcessStat {
	stats := make([]ProcessStat, 0)
	for _, pidStr := range pidList {
		pidNum, err := strconv.Atoi(pidStr)
		if err != nil {
			log.Fatal(err)
		}
		proc, err := getProcessByPID(pidNum)
		if err != nil {
			log.Fatal(err)
		}
		if proc == nil {
			continue
		}
		var stat ProcessStat
		stat.getProcessStat(proc)
		stats = append(stats, stat)
	}
	return stats
}

func getProcStatsByName() []ProcessStat {
	tiServers := []string{"pd-server", "tikv-server", "tidb-server"}
	stats := make([]ProcessStat, 0)
	for _, procName := range tiServers {
		procList, err := getProcessesByName(procName)
		if err != nil {
			log.Fatal(err)
		}
		if len(procList) < 1 {
			continue
		}
		for _, proc := range procList {
			var stat ProcessStat
			stat.getProcessStat(proc)
			stats = append(stats, stat)
		}
	}
	return stats
}

func getRlimitUsage(proc *process.Process) []RlimitUsage {
	resources := map[int32]string{
		// Resource limit constants are from:
		// /usr/include/x86_64-linux-gnu/bits/resource.h
		// from libc6-dev package in Ubuntu 16.10

		// Per-process CPU limit, in seconds.
		0: "cpu",

		// Largest file that can be created, in bytes.
		1: "fsize",

		// Maximum size of data segment, in bytes.
		2: "data",

		// Maximum size of stack segment, in bytes.
		3: "stack",

		// Largest core file that can be created, in bytes.
		4: "core",

		// Largest resident set size, in bytes.
		// This affects swapping; processes that are exceeding their
		// resident set size will be more likely to have physical memory
		// taken from them.
		5: "rss",

		// Number of processes.
		6: "nproc",

		// Number of open files.
		7: "nofile",

		// Locked-in-memory address space.
		8: "memlock",

		// Address space limit.
		9: "as",

		// Maximum number of file locks.
		10: "locks",

		// Maximum number of pending signals.
		11: "sigpending",

		// Maximum bytes in POSIX message queues.
		12: "msgqueue",

		// Maximum nice priority allowed to raise to.
		// Nice levels 19 .. -20 correspond to 0 .. 39
		// values of this resource limit.
		13: "nice",

		// Maximum realtime priority allowed for non-priviledged
		// processes.
		14: "rtprio",

		// Maximum CPU time in µs that a process scheduled under a real-time
		// scheduling policy may consume without making a blocking system
		// call before being forcibly descheduled.
		15: "rttime",
	}

	result := make([]RlimitUsage, 0)
	rlimit, _ := proc.RlimitUsage(true)
	for _, res := range rlimit {
		var usage RlimitUsage
		usage.Resource = resources[res.Resource]
		usage.Soft = int64(res.Soft)
		usage.Hard = int64(res.Hard)
		usage.Used = res.Used
		result = append(result, usage)
	}
	return result
}

//revive:disable:get-return
func (proc_stat *ProcessStat) getProcessStat(proc *process.Process) {
	proc_stat.Pid = proc.Pid
	proc_stat.Name, _ = proc.Name()
	proc_stat.Exec, _ = proc.Exe()
	proc_stat.Cmdline, _ = proc.Cmdline()
	proc_stat.Status, _ = proc.Status()
	proc_stat.StartTime, _ = getProcStartTime(proc)
	proc_stat.CPUTimes, _ = proc.Times()
	proc_stat.Memory, _ = proc.MemoryInfo()
	proc_stat.Rlimit = getRlimitUsage(proc)
}

//revive:enable:get-return

func getProcessByPID(pid int) (*process.Process, error) {
	procList, err := process.Processes()
	if err != nil || len(procList) < 1 {
		return nil, err
	}
	for _, proc := range procList {
		// skip when process no longer exist
		if int(proc.Pid) == pid {
			return proc, err
		}
	}
	return nil, err
}

func getProcessesByName(searchName string) ([]*process.Process, error) {
	procList, err := process.Processes()
	if err != nil || len(procList) < 1 {
		return nil, err
	}

	procResult := make([]*process.Process, 0)
	for _, proc := range procList {
		// skip when process no longer exist
		procName, _ := proc.Name()
		// return multiple processes that match the search
		if strings.Contains(procName, searchName) {
			procResult = append(procResult, proc)
		}
	}
	return procResult, err
}
