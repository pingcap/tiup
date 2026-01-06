package proc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceNGMonitoring is the service ID for NG Monitoring.
	ServiceNGMonitoring ServiceID = "ng-monitoring"

	// ComponentNGMonitoring is the repository component ID for NG Monitoring.
	ComponentNGMonitoring RepoComponentID = "ng-monitoring"
)

func init() {
	RegisterComponentDisplayName(ComponentNGMonitoring, "NG Monitoring")
	RegisterServiceDisplayName(ServiceNGMonitoring, "NG Monitoring")
}

// NGMonitoringInstance represents a running ng-monitoring-server.
type NGMonitoringInstance struct {
	ProcessInfo

	PDs []*PDInstance
}

var _ Process = &NGMonitoringInstance{}

// LogFile returns the log file path for the instance.
func (inst *NGMonitoringInstance) LogFile() string {
	return filepath.Join(inst.Dir, "ng-monitoring.log")
}

func resolveSiblingBinary(baseBinPath, want string) (string, bool) {
	dir := filepath.Dir(baseBinPath)
	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, want)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(filepath.Dir(baseBinPath), want), false
}

// Prepare builds the NG Monitoring process command.
func (inst *NGMonitoringInstance) Prepare(ctx context.Context) error {
	if inst == nil {
		return errors.New("ng-monitoring instance is nil")
	}
	info := inst.Info()
	if inst.Dir == "" {
		return errors.New("ng-monitoring dir is empty")
	}
	if inst.BinPath == "" {
		return errors.New("ng-monitoring binary not resolved")
	}

	if err := utils.MkdirAll(inst.Dir, 0755); err != nil {
		return errors.AddStack(err)
	}

	binPath := inst.BinPath
	if filepath.Base(binPath) != "ng-monitoring-server" {
		if alt, ok := resolveSiblingBinary(binPath, "ng-monitoring-server"); ok {
			binPath = alt
		} else {
			return errors.Errorf("ng-monitoring-server not found near %s", inst.BinPath)
		}
	}

	var endpoints []string
	for _, pd := range inst.PDs {
		endpoints = append(endpoints, utils.JoinHostPort(pd.Host, pd.StatusPort))
	}

	addr := utils.JoinHostPort(inst.Host, inst.Port)
	args := []string{
		fmt.Sprintf("--pd.endpoints=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--address=%s", addr),
		fmt.Sprintf("--advertise-address=%s", addr),
		fmt.Sprintf("--storage.path=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("--log.path=%s", filepath.Join(inst.Dir, "logs")),
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, binPath, args, nil, inst.Dir)}
	return nil
}
