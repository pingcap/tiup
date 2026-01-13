package service

import (
	"fmt"
	"io"
	"net"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

func init() {
	startAfter := []proc.ServiceID{
		proc.ServicePD,
		proc.ServicePDAPI,
		proc.ServiceTiKV,
		proc.ServiceTiKVWorker,
		proc.ServicePump,
	}

	MustRegister(Spec{
		ServiceID: proc.ServiceTiDBSystem,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return newTiDBInstance(rt, proc.ServiceTiDBSystem, params)
		},
		Catalog: Catalog{
			FlagPrefix:      "db.system",
			AllowModifyNum:  true,
			MaxNum:          1,
			AllowModifyHost: true,
			AllowModifyPort: true,
			DefaultPort:     3000,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 3000, FromConfigPort: true},
				{Name: proc.PortNameStatusPort, Base: 10080, Host: "0.0.0.0"},
			},
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			DefaultNum: func(ctx BootContext) int {
				if ctx.SharedOptions().Mode == proc.ModeNextGen {
					return 1
				}
				return 0
			},
			DefaultBinPathFrom: proc.ServiceTiDB,
			IsEnabled:          func(ctx BootContext) bool { return ctx.SharedOptions().Mode == proc.ModeNextGen },
			IsCritical:         func(ctx BootContext) bool { return ctx.SharedOptions().Mode == proc.ModeNextGen },
		},
		StartAfter: startAfter,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentTiDB.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, baseConfigs map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)

			enableBinlog := false
			if c, ok := baseConfigs[proc.ServicePump]; ok && c.Num > 0 {
				enableBinlog = true
			}

			ws := byService[proc.ServiceTiKVWorker]
			tikvWorkerURLs := make([]string, 0, len(ws))
			for _, ws := range ws {
				tikvWorkerURLs = append(tikvWorkerURLs, utils.JoinHostPort(advertise(ws.Shared.Host), ws.Shared.Port))
			}

			for _, sp := range plans {
				sp.TiDB = &proc.TiDBPlan{
					PDAddrs:        pdBackendAddrs,
					EnableBinlog:   enableBinlog,
					TiKVWorkerURLs: tikvWorkerURLs,
				}
			}
			return nil
		},
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceTiDB,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return newTiDBInstance(rt, proc.ServiceTiDB, params)
		},
		Catalog: Catalog{
			FlagPrefix:      "db",
			AllowModifyNum:  true,
			AllowModifyHost: true,
			AllowModifyPort: true,
			DefaultPort:     4000,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 4000, FromConfigPort: true},
				{Name: proc.PortNameStatusPort, Base: 10080, Host: "0.0.0.0"},
			},
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			AllowModifyTimeout: true,
			DefaultTimeout:     60,
			DefaultNum: func(ctx BootContext) int {
				if ctx.SharedOptions().Mode == proc.ModeTiKVSlim {
					return 0
				}
				return 1
			},
			IsEnabled:     func(_ BootContext) bool { return true },
			IsCritical:    func(ctx BootContext) bool { return ctx.SharedOptions().Mode != proc.ModeTiKVSlim },
			AllowScaleOut: true,
		},
		StartAfter:   append([]proc.ServiceID{proc.ServiceTiDBSystem}, startAfter...),
		PostScaleOut: postScaleOutTiDB,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentTiDB.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, baseConfigs map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)

			enableBinlog := false
			if c, ok := baseConfigs[proc.ServicePump]; ok && c.Num > 0 {
				enableBinlog = true
			}

			for _, sp := range plans {
				sp.TiDB = &proc.TiDBPlan{
					PDAddrs:      pdBackendAddrs,
					EnableBinlog: enableBinlog,
				}
			}
			return nil
		},
	})
}

func enableBinlog(rt Runtime) bool {
	if rt == nil {
		return false
	}
	cfg, ok := rt.BootConfig(proc.ServicePump)
	return ok && cfg.Num > 0
}

func newTiDBInstance(rt ControllerRuntime, serviceID proc.ServiceID, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(serviceID, params, shOpt.PortOffset)
	if err != nil {
		return nil, err
	}

	pdAddrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		if pd == nil {
			continue
		}
		if addr := pd.Addr(); addr != "" {
			pdAddrs = append(pdAddrs, addr)
		}
	}

	var tikvWorkerURLs []string
	if serviceID == proc.ServiceTiDBSystem || rt.SharedOptions().Mode == proc.ModeCSE {
		kvwrks := ProcsOf[*proc.TiKVWorkerInstance](rt, proc.ServiceTiKVWorker)
		tikvWorkerURLs = make([]string, 0, len(kvwrks))
		for _, kvwrk := range kvwrks {
			tikvWorkerURLs = append(tikvWorkerURLs, utils.JoinHostPort(proc.AdvertiseHost(kvwrk.Host), kvwrk.Port))
		}
	}

	tdb := &proc.TiDBInstance{
		ShOpt:          shOpt,
		Plan:           proc.TiDBPlan{PDAddrs: pdAddrs, EnableBinlog: enableBinlog(rt), TiKVWorkerURLs: tikvWorkerURLs},
		TiProxyCertDir: rt.DataDir(),
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiDB,
			Service:         serviceID,
		},
	}
	tdb.UpTimeout = params.Config.UpTimeout
	rt.AddProc(serviceID, tdb)
	return tdb, nil
}

func postScaleOutTiDB(w io.Writer, inst proc.Process) {
	if w == nil || inst == nil {
		return
	}
	tdb, ok := inst.(*proc.TiDBInstance)
	if !ok {
		return
	}

	host, port, err := net.SplitHostPort(tdb.Addr())
	if err != nil {
		fmt.Fprintf(w, "TiDB is up at %s\n", tdb.Addr())
		return
	}
	fmt.Fprintf(w, "Connect TiDB: %s --host %s --port %s -u root\n", "mysql --comments", host, port)
}
