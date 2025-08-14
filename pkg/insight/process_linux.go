//go:build linux
// +build linux

package insight

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"
)

func getProcStartTime(proc *process.Process) (float64, error) {
	statPath := GetProcPath(strconv.Itoa(int(proc.Pid)), "stat")
	contents, err := os.ReadFile(statPath)
	if err != nil {
		log.Fatal(err)
		return 0, err
	}
	fields := strings.Fields(string(contents))
	if startTime, err := strconv.ParseFloat(fields[21], 64); err == nil {
		return startTime / float64(process.ClockTicks), err
	}
	return 0, err
}
