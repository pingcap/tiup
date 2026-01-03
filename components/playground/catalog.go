package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

type serviceDef struct {
	ServiceID proc.ServiceID

	FlagPrefix string

	HasCount       bool
	HasHost        bool
	HasPort        bool
	PortDefault    int
	HasConfig      bool
	HasBinPath     bool
	HasTimeout     bool
	TimeoutDefault int
	HasVersion     bool

	ScaleOut bool

	DefaultNum func(o *BootOptions) int

	// DefaultXXXFrom copies the value from another service when the destination
	// flag is not explicitly set.
	DefaultNumFrom        proc.ServiceID
	DefaultBinPathFrom    proc.ServiceID
	DefaultConfigPathFrom proc.ServiceID
	DefaultTimeoutFrom    proc.ServiceID

	// CopyFrom copies the entire proc.Config from another service unconditionally
	// when CopyWhen returns true.
	CopyFrom proc.ServiceID
	CopyWhen func(o *BootOptions) bool

	PlanWhen   func(o *BootOptions) bool
	PlanConfig func(o *BootOptions) proc.Config

	CriticalWhen func(o *BootOptions) bool
}

func (d serviceDef) displayName() string {
	if d.ServiceID != "" {
		if s := proc.ServiceDisplayName(d.ServiceID); s != "" {
			return s
		}
		return d.ServiceID.String()
	}
	return "service"
}

func (d serviceDef) countFlag() string   { return d.FlagPrefix }
func (d serviceDef) hostFlag() string    { return d.FlagPrefix + ".host" }
func (d serviceDef) portFlag() string    { return d.FlagPrefix + ".port" }
func (d serviceDef) configFlag() string  { return d.FlagPrefix + ".config" }
func (d serviceDef) binPathFlag() string { return d.FlagPrefix + ".binpath" }
func (d serviceDef) timeoutFlag() string { return d.FlagPrefix + ".timeout" }
func (d serviceDef) versionFlag() string { return d.FlagPrefix + ".version" }

func (d serviceDef) helpCount() string {
	return fmt.Sprintf("%s instance number", d.displayName())
}

func (d serviceDef) helpHost() string {
	return fmt.Sprintf("%s host (default: --host)", d.displayName())
}

func (d serviceDef) helpPort() string {
	return fmt.Sprintf("%s port (0 means default)", d.displayName())
}

func (d serviceDef) helpConfig() string {
	return fmt.Sprintf("%s instance configuration file", d.displayName())
}

func (d serviceDef) helpBinPath() string {
	return fmt.Sprintf("%s instance binary path", d.displayName())
}

func (d serviceDef) helpTimeout() string {
	return fmt.Sprintf("%s max wait time in seconds for starting, 0 means no limit", d.displayName())
}

func (d serviceDef) helpVersion() string {
	return fmt.Sprintf("%s instance version (override)", d.displayName())
}

var serviceCatalog = []serviceDef{
	// PD (single-service).
	{
		ServiceID:    proc.ServicePD,
		FlagPrefix:   "pd",
		HasCount:     true,
		HasHost:      true,
		HasPort:      true,
		HasConfig:    true,
		HasBinPath:   true,
		DefaultNum:   func(o *BootOptions) int { return 1 },
		PlanWhen:     func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode != "ms" },
		CriticalWhen: func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode != "ms" },
		ScaleOut:     true,
	},
	// PD (microservices API service) - configured by --pd.* when pd.mode=ms.
	{
		ServiceID:    proc.ServicePDAPI,
		ScaleOut:     true,
		PlanWhen:     func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		CopyFrom:     proc.ServicePD,
		CopyWhen:     func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		CriticalWhen: func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
	},
	{
		ServiceID:  proc.ServicePDTSO,
		FlagPrefix: "tso",
		HasCount:   true,
		HasHost:    true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int {
			if o != nil && o.ShOpt.PDMode == "ms" {
				return 1
			}
			return 0
		},
		DefaultBinPathFrom:    proc.ServicePD,
		DefaultConfigPathFrom: proc.ServicePD,
		PlanWhen:              func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		CriticalWhen:          func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		ScaleOut:              true,
	},
	{
		ServiceID:  proc.ServicePDScheduling,
		FlagPrefix: "scheduling",
		HasCount:   true,
		HasHost:    true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int {
			if o != nil && o.ShOpt.PDMode == "ms" {
				return 1
			}
			return 0
		},
		DefaultBinPathFrom:    proc.ServicePD,
		DefaultConfigPathFrom: proc.ServicePD,
		PlanWhen:              func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		CriticalWhen:          func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		ScaleOut:              true,
	},
	{
		ServiceID:             proc.ServicePDRouter,
		FlagPrefix:            "router",
		HasCount:              true,
		HasHost:               true,
		HasConfig:             true,
		HasBinPath:            true,
		DefaultNum:            func(o *BootOptions) int { return 0 },
		DefaultBinPathFrom:    proc.ServicePD,
		DefaultConfigPathFrom: proc.ServicePD,
		PlanWhen:              func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		ScaleOut:              true,
	},
	{
		ServiceID:             proc.ServicePDResourceManager,
		FlagPrefix:            "resource-manager",
		HasCount:              true,
		HasHost:               true,
		HasConfig:             true,
		HasBinPath:            true,
		DefaultNum:            func(o *BootOptions) int { return 0 },
		DefaultBinPathFrom:    proc.ServicePD,
		DefaultConfigPathFrom: proc.ServicePD,
		PlanWhen:              func(o *BootOptions) bool { return o != nil && o.ShOpt.PDMode == "ms" },
		ScaleOut:              true,
	},

	// Storage.
	{
		ServiceID:    proc.ServiceTiKV,
		FlagPrefix:   "kv",
		HasCount:     true,
		HasHost:      true,
		HasPort:      true,
		HasConfig:    true,
		HasBinPath:   true,
		DefaultNum:   func(o *BootOptions) int { return 1 },
		PlanWhen:     func(o *BootOptions) bool { return true },
		CriticalWhen: func(o *BootOptions) bool { return true },
		ScaleOut:     true,
	},
	{
		ServiceID:   proc.ServiceTiKVWorker,
		FlagPrefix:  "tikv.worker",
		HasCount:    true,
		HasHost:     true,
		HasPort:     true,
		PortDefault: 19000,
		HasConfig:   true,
		HasBinPath:  true,
		DefaultNum: func(o *BootOptions) int {
			if o == nil {
				return 0
			}
			switch o.ShOpt.Mode {
			case proc.ModeNextGen, proc.ModeCSE, proc.ModeDisAgg:
				return 1
			default:
				return 0
			}
		},
		DefaultBinPathFrom: proc.ServiceTiKV,
		PlanWhen: func(o *BootOptions) bool {
			return o != nil && (o.ShOpt.Mode == proc.ModeNextGen || o.ShOpt.Mode == proc.ModeCSE || o.ShOpt.Mode == proc.ModeDisAgg)
		},
		CriticalWhen: func(o *BootOptions) bool { return true },
	},

	// Binlog (optional).
	{
		ServiceID:  proc.ServicePump,
		FlagPrefix: "pump",
		HasCount:   true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int { return 0 },
		PlanWhen:   func(o *BootOptions) bool { return true },
		ScaleOut:   true,
	},

	// TiDB.
	{
		ServiceID:  proc.ServiceTiDBSystem,
		FlagPrefix: "db.system",
		HasCount:   true,
		HasHost:    true,
		HasPort:    true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int {
			if o != nil && o.ShOpt.Mode == proc.ModeNextGen {
				return 1
			}
			return 0
		},
		DefaultBinPathFrom: proc.ServiceTiDB,
		PlanWhen:           func(o *BootOptions) bool { return o != nil && o.ShOpt.Mode == proc.ModeNextGen },
		CriticalWhen:       func(o *BootOptions) bool { return o != nil && o.ShOpt.Mode == proc.ModeNextGen },
	},
	{
		ServiceID:      proc.ServiceTiDB,
		FlagPrefix:     "db",
		HasCount:       true,
		HasHost:        true,
		HasPort:        true,
		HasConfig:      true,
		HasBinPath:     true,
		HasTimeout:     true,
		TimeoutDefault: 60,
		DefaultNum: func(o *BootOptions) int {
			if o != nil && o.ShOpt.Mode == proc.ModeTiKVSlim {
				return 0
			}
			return 1
		},
		PlanWhen:     func(o *BootOptions) bool { return true },
		CriticalWhen: func(o *BootOptions) bool { return o != nil && o.ShOpt.Mode != proc.ModeTiKVSlim },
		ScaleOut:     true,
	},

	// Optional ecosystem services.
	{
		ServiceID:      proc.ServiceTiProxy,
		FlagPrefix:     "tiproxy",
		HasCount:       true,
		HasHost:        true,
		HasPort:        true,
		HasConfig:      true,
		HasBinPath:     true,
		HasTimeout:     true,
		TimeoutDefault: 60,
		HasVersion:     true,
		DefaultNum:     func(o *BootOptions) int { return 0 },
		PlanWhen:       func(o *BootOptions) bool { return true },
		ScaleOut:       true,
	},
	{
		ServiceID:  proc.ServiceTiCDC,
		FlagPrefix: "ticdc",
		HasCount:   true,
		HasHost:    true,
		HasPort:    true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int { return 0 },
		PlanWhen:   func(o *BootOptions) bool { return true },
		ScaleOut:   true,
	},
	{
		ServiceID:  proc.ServiceTiKVCDC,
		FlagPrefix: "kvcdc",
		HasCount:   true,
		HasConfig:  true,
		HasBinPath: true,
		HasVersion: true,
		DefaultNum: func(o *BootOptions) int { return 0 },
		PlanWhen:   func(o *BootOptions) bool { return true },
		ScaleOut:   true,
	},
	{
		ServiceID:  proc.ServiceDrainer,
		FlagPrefix: "drainer",
		HasCount:   true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int { return 0 },
		PlanWhen:   func(o *BootOptions) bool { return true },
		ScaleOut:   true,
	},
	{
		ServiceID:   proc.ServiceDMMaster,
		FlagPrefix:  "dm-master",
		HasCount:    true,
		HasHost:     true,
		HasPort:     true,
		PortDefault: 8261,
		HasConfig:   true,
		HasBinPath:  true,
		DefaultNum:  func(o *BootOptions) int { return 0 },
		PlanWhen:    func(o *BootOptions) bool { return true },
		ScaleOut:    true,
	},
	{
		ServiceID:   proc.ServiceDMWorker,
		FlagPrefix:  "dm-worker",
		HasCount:    true,
		HasHost:     true,
		HasPort:     true,
		PortDefault: 8262,
		HasConfig:   true,
		HasBinPath:  true,
		DefaultNum:  func(o *BootOptions) int { return 0 },
		PlanWhen:    func(o *BootOptions) bool { return true },
		ScaleOut:    true,
	},

	// TiFlash.
	{
		ServiceID:      proc.ServiceTiFlash,
		FlagPrefix:     "tiflash",
		HasCount:       true,
		HasConfig:      true,
		HasBinPath:     true,
		HasTimeout:     true,
		TimeoutDefault: 120,
		DefaultNum: func(o *BootOptions) int {
			if o == nil {
				return 0
			}
			switch o.ShOpt.Mode {
			case proc.ModeNormal, proc.ModeCSE, proc.ModeDisAgg:
				return 1
			default:
				return 0
			}
		},
		PlanWhen: func(o *BootOptions) bool { return o != nil && o.ShOpt.Mode == proc.ModeNormal },
		ScaleOut: true,
	},
	{
		ServiceID:  proc.ServiceTiFlashWrite,
		FlagPrefix: "tiflash.write",
		HasCount:   true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int {
			if o == nil {
				return 0
			}
			switch o.ShOpt.Mode {
			case proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg:
				return o.Service(proc.ServiceTiFlash).Num
			default:
				return 0
			}
		},
		DefaultBinPathFrom:    proc.ServiceTiFlash,
		DefaultConfigPathFrom: proc.ServiceTiFlash,
		DefaultTimeoutFrom:    proc.ServiceTiFlash,
		PlanWhen: func(o *BootOptions) bool {
			return o != nil && (o.ShOpt.Mode == proc.ModeCSE || o.ShOpt.Mode == proc.ModeNextGen || o.ShOpt.Mode == proc.ModeDisAgg)
		},
	},
	{
		ServiceID:  proc.ServiceTiFlashCompute,
		FlagPrefix: "tiflash.compute",
		HasCount:   true,
		HasConfig:  true,
		HasBinPath: true,
		DefaultNum: func(o *BootOptions) int {
			if o == nil {
				return 0
			}
			switch o.ShOpt.Mode {
			case proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg:
				return o.Service(proc.ServiceTiFlash).Num
			default:
				return 0
			}
		},
		DefaultBinPathFrom:    proc.ServiceTiFlash,
		DefaultConfigPathFrom: proc.ServiceTiFlash,
		DefaultTimeoutFrom:    proc.ServiceTiFlash,
		PlanWhen: func(o *BootOptions) bool {
			return o != nil && (o.ShOpt.Mode == proc.ModeCSE || o.ShOpt.Mode == proc.ModeNextGen || o.ShOpt.Mode == proc.ModeDisAgg)
		},
	},

	// Monitoring (controlled by global --monitor/--without-monitor).
	{
		ServiceID: proc.ServicePrometheus,
		PlanWhen:  func(o *BootOptions) bool { return o != nil && o.Monitor },
		PlanConfig: func(o *BootOptions) proc.Config {
			return proc.Config{Num: 1}
		},
	},
	{
		ServiceID: proc.ServiceGrafana,
		PlanWhen:  func(o *BootOptions) bool { return o != nil && o.Monitor },
		PlanConfig: func(o *BootOptions) proc.Config {
			port := 0
			if o != nil {
				port = o.GrafanaPort
			}
			return proc.Config{Num: 1, Port: port}
		},
	},
	{
		ServiceID: proc.ServiceNGMonitoring,
		PlanWhen: func(o *BootOptions) bool {
			if o == nil || !o.Monitor {
				return false
			}
			// ng-monitoring-server is only available in newer releases. Skip it on
			// versions known to not include it.
			baseVersion := strings.TrimSuffix(o.Version, "-"+utils.NextgenVersionAlias)
			if !utils.Version(baseVersion).IsValid() {
				return true
			}
			return semver.Compare(baseVersion, "v5.3.0") >= 0
		},
		PlanConfig: func(o *BootOptions) proc.Config {
			return proc.Config{Num: 1}
		},
	},
}

func serviceDefFor(serviceID proc.ServiceID) (serviceDef, bool) {
	if serviceID == "" {
		return serviceDef{}, false
	}
	for _, def := range serviceCatalog {
		if def.ServiceID == serviceID {
			return def, true
		}
	}
	return serviceDef{}, false
}

func scaleOutServiceIDs() []proc.ServiceID {
	var out []proc.ServiceID
	for _, def := range serviceCatalog {
		if def.ServiceID == "" || !def.ScaleOut {
			continue
		}
		out = append(out, def.ServiceID)
	}
	slices.SortFunc(out, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})
	return out
}
