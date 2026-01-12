package proc

import (
	"strings"

	"github.com/pingcap/errors"
)

type plannedProcessFactory func(plan ServicePlan, info ProcessInfo, shOpt SharedOptions, dataDir string) (Process, error)

var plannedProcessFactories map[ServiceID]plannedProcessFactory

func registerPlannedProcessFactory(serviceID ServiceID, factory plannedProcessFactory) {
	if serviceID == "" {
		panic("planned process factory: service id is empty")
	}
	if factory == nil {
		panic("planned process factory: factory is nil for " + serviceID.String())
	}

	if plannedProcessFactories == nil {
		plannedProcessFactories = make(map[ServiceID]plannedProcessFactory)
	}
	if _, exists := plannedProcessFactories[serviceID]; exists {
		panic("planned process factory already registered for " + serviceID.String())
	}
	plannedProcessFactories[serviceID] = factory
}

// NewProcessFromPlan creates a proc.Process instance from a planned service entry.
//
// It is a centralized mapping from service IDs to proc implementations, so the
// controller does not need to maintain a parallel "service -> proc type" switch.
//
// NOTE: This function does not validate ports/dirs beyond what is necessary for
// process construction. Planner and controller are responsible for those checks.
func NewProcessFromPlan(plan ServicePlan, info ProcessInfo, shOpt SharedOptions, dataDir string) (Process, error) {
	serviceID := info.Service
	if serviceID == "" {
		return nil, errors.New("planned service id is empty")
	}

	if pid := strings.TrimSpace(plan.ServiceID); pid != "" && ServiceID(pid) != serviceID {
		return nil, errors.Errorf("planned service id mismatch: plan=%q info=%q", plan.ServiceID, serviceID)
	}

	factory := plannedProcessFactories[serviceID]
	if factory == nil {
		return nil, errors.Errorf("unsupported service %s", serviceID)
	}
	return factory(plan, info, shOpt, dataDir)
}
