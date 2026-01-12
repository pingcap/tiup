package proc

import (
	"context"
	"fmt"
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

	registerPlannedProcessFactory(ServiceNGMonitoring, func(plan ServicePlan, info ProcessInfo, _ SharedOptions, _ string) (Process, error) {
		if plan.NGMonitoring == nil {
			name := info.Name()
			if name == "" {
				name = ServiceNGMonitoring.String()
			}
			return nil, errors.Errorf("missing ng-monitoring plan for %s", name)
		}
		return &NGMonitoringInstance{Plan: *plan.NGMonitoring, ProcessInfo: info}, nil
	})
}

// NGMonitoringPlan is the service-specific plan for NG Monitoring.
type NGMonitoringPlan struct {
	PDAddrs []string
}

// NGMonitoringInstance represents a running ng-monitoring-server.
type NGMonitoringInstance struct {
	ProcessInfo

	Plan NGMonitoringPlan
}

var _ Process = &NGMonitoringInstance{}

// LogFile returns the log file path for the instance.
func (inst *NGMonitoringInstance) LogFile() string {
	return filepath.Join(inst.Dir, "ng-monitoring.log")
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
		if alt, ok := ResolveSiblingBinary(binPath, "ng-monitoring-server"); ok {
			binPath = alt
		} else {
			return errors.Errorf("ng-monitoring-server not found near %s", inst.BinPath)
		}
	}

	endpoints := append([]string(nil), inst.Plan.PDAddrs...)

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
