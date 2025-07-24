// tidb-insight project tikv.go
package insight

import (
	"bytes"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"
)

// TiKVMeta is the metada struct of a TiKV server
type TiKVMeta struct {
	Pid         int32  `json:"pid,omitempty"`
	ReleaseVer  string `json:"release_version,omitempty"`
	GitCommit   string `json:"git_commit,omitempty"`
	GitBranch   string `json:"git_branch,omitempty"`
	BuildTime   string `json:"utc_build_time,omitempty"`
	RustVersion string `json:"rust_version,omitempty"`
}

func getTiKVVersion(proc *process.Process) TiKVMeta {
	var tikvVer TiKVMeta
	tikvVer.Pid = proc.Pid
	file, err := proc.Exe()
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(file, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	output := strings.Split(out.String(), "\n")
	for _, line := range output {
		info := strings.Split(line, ":")
		if len(info) <= 1 {
			continue
		}
		switch info[0] {
		case "Release Version":
			tikvVer.ReleaseVer = strings.TrimSpace(info[1])
		case "Git Commit Hash":
			tikvVer.GitCommit = strings.TrimSpace(info[1])
		case "Git Commit Branch":
			tikvVer.GitBranch = strings.TrimSpace(info[1])
		case "UTC Build Time":
			tikvVer.BuildTime = strings.TrimSpace(strings.Join(info[1:], ":"))
		case "Rust Version":
			tikvVer.RustVersion = strings.TrimSpace(info[1])
		default:
			continue
		}
	}

	return tikvVer
}

func getTiKVVersionByName() []TiKVMeta {
	var tikvMeta = make([]TiKVMeta, 0)
	procList, err := getProcessesByName("tikv-server")
	if err != nil {
		log.Fatal(err)
	}
	if len(procList) < 1 {
		return tikvMeta
	}

	for _, proc := range procList {
		tikvMeta = append(tikvMeta, getTiKVVersion(proc))
	}
	return tikvMeta
}

func getTiKVVersionByPIDList(pidList []string) []TiKVMeta {
	tikvMeta := make([]TiKVMeta, 0)
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
		procName, _ := proc.Name()
		if !strings.Contains(procName, "tikv-server") {
			continue
		}
		tikvMeta = append(tikvMeta, getTiKVVersion(proc))
	}
	return tikvMeta
}
