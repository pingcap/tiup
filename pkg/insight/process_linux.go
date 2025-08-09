//go:build linux
// +build linux

package insight

import (
	"log"

	"github.com/shirou/gopsutil/process"
)

func getProcStartTime(proc *process.Process) (float64, error) {
	createTimeMs, err := proc.CreateTime()
	if err != nil {
		log.Fatal(err) // Matches existing error handling behavior
		return 0, err
	}
	return float64(createTimeMs) / 1000.0, nil // Convert milliseconds to seconds
}
