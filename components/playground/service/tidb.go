package service

import (
	"fmt"
	"io"
	"net"

	"github.com/pingcap/tiup/components/playground/proc"
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
			FlagPrefix:         "db.system",
			AllowModifyNum:     true,
			MaxNum:             1,
			AllowModifyHost:    true,
			AllowModifyPort:    true,
			DefaultPort:        3000,
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
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceTiDB,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return newTiDBInstance(rt, proc.ServiceTiDB, params)
		},
		Catalog: Catalog{
			FlagPrefix:         "db",
			AllowModifyNum:     true,
			AllowModifyHost:    true,
			AllowModifyPort:    true,
			DefaultPort:        4000,
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
	kvwrks := ProcsOf[*proc.TiKVWorkerInstance](rt, proc.ServiceTiKVWorker)
	shOpt := rt.SharedOptions()
	defaultPort := 4000
	if serviceID == proc.ServiceTiDBSystem {
		defaultPort = 3000
	}
	tdb := &proc.TiDBInstance{
		ShOpt:          shOpt,
		PDs:            pds,
		KVWorkers:      kvwrks,
		TiProxyCertDir: rt.DataDir(),
		EnableBinlog:   enableBinlog(rt),
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, params.Config.Port, defaultPort, shOpt.PortOffset),
			StatusPort:      allocPort("0.0.0.0", 0, 10080, shOpt.PortOffset),
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
