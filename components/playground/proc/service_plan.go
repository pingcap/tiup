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
	PD           *PDPlan
	TiKV         *TiKVPlan
	TiDB         *TiDBPlan
	TiKVWorker   *TiKVWorkerPlan
	TiFlash      *TiFlashPlan
	TiProxy      *TiProxyPlan
	Grafana      *GrafanaPlan
	NGMonitoring *NGMonitoringPlan
	TiCDC        *TiCDCPlan
	TiKVCDC      *TiKVCDCPlan
	DMMaster     *DMMasterPlan
	DMWorker     *DMWorkerPlan
	Pump         *PumpPlan
	Drainer      *DrainerPlan

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
