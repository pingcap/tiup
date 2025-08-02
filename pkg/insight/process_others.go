//go:build !linux
// +build !linux

package insight

import (
	"github.com/shirou/gopsutil/process"
)

func getProcStartTime(proc *process.Process) (float64, error) {
	return 0, nil
}
