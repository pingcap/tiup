package proc

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
)

// ServicePlan is the per-instance start plan produced by the playground
// planner.
//
// It intentionally omits low-level details like argv/workdir/env/fileops.
// Executor is responsible for turning the plan into concrete process commands.
type ServicePlan struct {
	Name      string
	ServiceID string

	StartAfterServices []string

	ComponentID     string
	ResolvedVersion string
	BinPath         string

	Shared ServiceSharedPlan

	// Service-specific inputs (strong schema).
	PD           *PDPlan           `json:",omitempty"`
	TiKV         *TiKVPlan         `json:",omitempty"`
	TiDB         *TiDBPlan         `json:",omitempty"`
	TiKVWorker   *TiKVWorkerPlan   `json:",omitempty"`
	TiFlash      *TiFlashPlan      `json:",omitempty"`
	TiProxy      *TiProxyPlan      `json:",omitempty"`
	Grafana      *GrafanaPlan      `json:",omitempty"`
	NGMonitoring *NGMonitoringPlan `json:",omitempty"`
	TiCDC        *TiCDCPlan        `json:",omitempty"`
	TiKVCDC      *TiKVCDCPlan      `json:",omitempty"`
	DMMaster     *DMMasterPlan     `json:",omitempty"`
	DMWorker     *DMWorkerPlan     `json:",omitempty"`
	Pump         *PumpPlan         `json:",omitempty"`
	Drainer      *DrainerPlan      `json:",omitempty"`

	// Debug fields (not part of execution semantics).
	DebugConstraint string
}

const (
	// PortNamePort is the standard key for the main listen port in
	// ServiceSharedPlan.Ports.
	PortNamePort = "port"
	// PortNameStatusPort is the standard key for the status/metrics port in
	// ServiceSharedPlan.Ports.
	PortNameStatusPort = "statusPort"
)

// ServiceSharedPlan contains common, low-level per-instance inputs.
type ServiceSharedPlan struct {
	Dir        string
	Host       string
	Port       int
	StatusPort int

	// Ports holds additional named ports allocated during planning.
	//
	// It is a low-level, per-instance detail: planner fills it based on service
	// catalog definitions and executor/proc implementations may read it when a
	// service needs more than the standard Port/StatusPort pair (e.g. TiFlash).
	Ports map[string]int `json:",omitempty"`

	ConfigPath string
	UpTimeout  int
}

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

// ResolveSiblingBinary searches for an executable named want near baseBinPath.
//
// It checks the directory containing baseBinPath and up to 3 parent directories.
// The returned bool reports whether the binary exists.
func ResolveSiblingBinary(baseBinPath, want string) (string, bool) {
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
