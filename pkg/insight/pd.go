// tidb-insight project pd.go
package insight

import (
	"bytes"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"
)

// PDMeta is the metadata struct of a PD server
type PDMeta struct {
	Pid        int32  `json:"pid,omitempty"`
	ReleaseVer string `json:"release_version,omitempty"`
	GitCommit  string `json:"git_commit,omitempty"`
	GitBranch  string `json:"git_branch,omitempty"`
	BuildTime  string `json:"utc_build_time,omitempty"`
}

func getPDVersion(proc *process.Process) PDMeta {
	var pdVer PDMeta
	pdVer.Pid = proc.Pid
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
			pdVer.ReleaseVer = strings.TrimSpace(info[1])
		case "Git Commit Hash":
			pdVer.GitCommit = strings.TrimSpace(info[1])
		case "Git Branch":
			pdVer.GitBranch = strings.TrimSpace(info[1])
		case "UTC Build Time":
			pdVer.BuildTime = strings.TrimSpace(strings.Join(info[1:], ":"))
		default:
			continue
		}
	}

	return pdVer
}

func getPDVersionByName() []PDMeta {
	var pdMeta = make([]PDMeta, 0)
	procList, err := getProcessesByName("pd-server")
	if err != nil {
		log.Fatal(err)
	}
	if len(procList) < 1 {
		return pdMeta
	}

	for _, proc := range procList {
		pdMeta = append(pdMeta, getPDVersion(proc))
	}
	return pdMeta
}

func getPDVersionByPIDList(pidList []string) []PDMeta {
	pdMeta := make([]PDMeta, 0)
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
		if !strings.Contains(procName, "pd-server") {
			continue
		}
		pdMeta = append(pdMeta, getPDVersion(proc))
	}
	return pdMeta
}
