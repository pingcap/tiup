package proc

// PDPlan is the service-specific plan for PD-related services.
//
// Fields are intentionally "static": executor should be able to start instances
// without re-deriving endpoints from other instances.
type PDPlan struct {
	// InitialCluster is used by pd/pd-api (choose one of InitialCluster/JoinAddrs).
	InitialCluster []PDMemberPlan
	// JoinAddrs is used by pd/pd-api when joining an existing cluster.
	JoinAddrs []string // host:peerPort

	// BackendAddrs is used by pd-* microservices as backend endpoints.
	BackendAddrs []string // host:statusPort

	KVIsSingleReplica bool
}

// PDMemberPlan is one member in the pd/pd-api initial cluster.
type PDMemberPlan struct {
	Name     string
	PeerAddr string // host:peerPort
}

// TiKVPlan is the service-specific plan for TiKV.
type TiKVPlan struct {
	PDAddrs  []string // host:statusPort
	TSOAddrs []string // host:statusPort (ms mode)
}

// TiDBPlan is the service-specific plan for TiDB.
type TiDBPlan struct {
	PDAddrs []string

	EnableBinlog bool

	// TiKVWorkerURL is only used by tidb-system in NextGen mode.
	TiKVWorkerURL string // http://host:port
}

// TiKVWorkerPlan is the service-specific plan for TiKV worker.
type TiKVWorkerPlan struct {
	PDAddrs []string
}

// TiFlashPlan is the service-specific plan for TiFlash.
type TiFlashPlan struct {
	PDAddrs []string

	// ProcessInfo.Port remains the TiFlash HTTP port.
	ServicePort     int
	TCPPort         int
	ProxyPort       int
	ProxyStatusPort int
}

// TiProxyPlan is the service-specific plan for TiProxy.
type TiProxyPlan struct {
	PDAddrs []string
}

// NGMonitoringPlan is the service-specific plan for NG Monitoring.
type NGMonitoringPlan struct {
	PDAddrs []string
}

// GrafanaPlan is the service-specific plan for Grafana.
type GrafanaPlan struct {
	PrometheusURL string // http://host:port
}

// TiCDCPlan is the service-specific plan for TiCDC.
type TiCDCPlan struct{ PDAddrs []string }

// TiKVCDCPlan is the service-specific plan for TiKV-CDC.
type TiKVCDCPlan struct{ PDAddrs []string }

// PumpPlan is the service-specific plan for Pump.
type PumpPlan struct{ PDAddrs []string }

// DrainerPlan is the service-specific plan for Drainer.
type DrainerPlan struct{ PDAddrs []string }

// DMMasterPlan is the service-specific plan for DM-master.
type DMMasterPlan struct {
	InitialCluster []DMMemberPlan
	RequireReady   bool
}

// DMMemberPlan is one member in the DM initial cluster.
type DMMemberPlan struct {
	Name       string
	PeerAddr   string // host:peerPort
	MasterAddr string // host:statusPort
}

// DMWorkerPlan is the service-specific plan for DM-worker.
type DMWorkerPlan struct {
	MasterAddrs []string // host:statusPort
}
