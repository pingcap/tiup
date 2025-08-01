package insight

import (
	"os"
	"strconv"
	"strings"
)

// Version information
var (
	// GitBranch is initialized during make
	GitBranch = "Not Provided"

	// GitCommit is initialized during make
	GitCommit = "Not Provided"

	// Proc dir path for Linux
	procPath = "/proc"
)

// GetProcPath is getting the proc path
func GetProcPath(paths ...string) string {
	switch len(paths) {
	case 0:
		return procPath
	default:
		all := make([]string, len(paths)+1)
		all[0] = procPath
		copy(all[1:], paths)
		return strings.Join(all, "/")
	}
}

// GetSysUptime gets the system uptime
func GetSysUptime() (float64, float64, error) {
	contents, err := os.ReadFile(GetProcPath("uptime"))
	if err != nil {
		return 0, 0, err
	}
	timerCounts := strings.Fields(string(contents))
	uptime, err := strconv.ParseFloat(timerCounts[0], 64)
	if err != nil {
		return 0, 0, err
	}
	idleTime, err := strconv.ParseFloat(timerCounts[1], 64)
	if err != nil {
		return 0, 0, err
	}
	return uptime, idleTime, err
}
