// tidb-insight project tidb.go
package insight

import (
	"bytes"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"
)

// TiDBMeta is the metadata struct of TiDB server
type TiDBMeta struct {
	Pid        int32  `json:"pid,omitempty"`
	ReleaseVer string `json:"release_version,omitempty"`
	GitCommit  string `json:"git_commit,omitempty"`
	GitBranch  string `json:"git_branch,omitempty"`
	BuildTime  string `json:"utc_build_time,omitempty"`
	GoVersion  string `json:"go_version,omitempty"`
}

func getTiDBVersion(proc *process.Process) TiDBMeta {
	var tidbVer TiDBMeta
	tidbVer.Pid = proc.Pid
	file, err := proc.Exe()
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(file, "-V")
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
			tidbVer.ReleaseVer = strings.TrimSpace(info[1])
		case "Git Commit Hash":
			tidbVer.GitCommit = strings.TrimSpace(info[1])
		case "Git Commit Branch":
			tidbVer.GitBranch = strings.TrimSpace(info[1])
		case "UTC Build Time":
			tidbVer.BuildTime = strings.TrimSpace(strings.Join(info[1:], ":"))
		case "GoVersion":
			infoTrimed := strings.TrimSpace(info[1])
			tidbVer.GoVersion = strings.TrimPrefix(infoTrimed, "go version ")
		default:
			continue
		}
	}
	return tidbVer
}

func getTiDBVersionByName() []TiDBMeta {
	var tidbMeta = make([]TiDBMeta, 0)
	procList, err := getProcessesByName("tidb-server")
	if err != nil {
		log.Fatal(err)
	}
	if len(procList) < 1 {
		return tidbMeta
	}

	for _, proc := range procList {
		tidbMeta = append(tidbMeta, getTiDBVersion(proc))
	}
	return tidbMeta
}

func getTiDBVersionByPIDList(pidList []string) []TiDBMeta {
	tidbMeta := make([]TiDBMeta, 0)
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
		if !strings.Contains(procName, "tidb-server") {
			continue
		}
		tidbMeta = append(tidbMeta, getTiDBVersion(proc))
	}
	return tidbMeta
}
