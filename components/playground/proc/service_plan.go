package proc

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

// ServiceSharedPlan contains common, low-level per-instance inputs.
type ServiceSharedPlan struct {
	Dir        string
	Host       string
	Port       int
	StatusPort int

	ConfigPath string
	UpTimeout  int
}
