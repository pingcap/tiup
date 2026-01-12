package service

import (
	"fmt"
	"io"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiKV,
		Catalog: Catalog{
			FlagPrefix:         "kv",
			AllowModifyNum:     true,
			AllowModifyHost:    true,
			AllowModifyPort:    true,
			DefaultPort:        20160,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			DefaultNum:         func(_ BootContext) int { return 1 },
			IsEnabled:          func(_ BootContext) bool { return true },
			IsCritical: func(_ BootContext) bool {
				return true
			},
			AllowScaleOut: true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
			proc.ServicePDTSO,
		},
		NewProc:     newTiKVInstance,
		ScaleInHook: scaleInTiKVByTombstone,
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceTiKVWorker,
		Catalog: Catalog{
			FlagPrefix:         "kv.worker",
			AllowModifyNum:     true,
			MaxNum:             1,
			AllowModifyHost:    true,
			AllowModifyPort:    true,
			DefaultPort:        19000,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			DefaultNum: func(ctx BootContext) int {
				switch ctx.SharedOptions().Mode {
				case proc.ModeNextGen, proc.ModeCSE:
					return 1
				default:
					return 0
				}
			},
			DefaultBinPathFrom: proc.ServiceTiKV,
			IsEnabled: func(ctx BootContext) bool {
				switch ctx.SharedOptions().Mode {
				case proc.ModeNextGen, proc.ModeCSE:
					return true
				default:
					return false
				}
			},
			IsCritical: func(_ BootContext) bool { return true },
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: newTiKVWorkerInstance,
	})
}

func newTiKVInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	tsos := ProcsOf[*proc.PDInstance](rt, proc.ServicePDTSO)
	shOpt := rt.SharedOptions()

	pdAddrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		if pd == nil {
			continue
		}
		if addr := pd.Addr(); addr != "" {
			pdAddrs = append(pdAddrs, addr)
		}
	}

	tsoAddrs := make([]string, 0, len(tsos))
	for _, tso := range tsos {
		if tso == nil {
			continue
		}
		if addr := tso.Addr(); addr != "" {
			tsoAddrs = append(tsoAddrs, addr)
		}
	}

	kv := &proc.TiKVInstance{
		ShOpt: shOpt,
		Plan: proc.TiKVPlan{
			PDAddrs:  pdAddrs,
			TSOAddrs: tsoAddrs,
		},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, params.Config.Port, 20160, shOpt.PortOffset),
			StatusPort:      allocPort(params.Host, 0, 20180, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiKV,
			Service:         proc.ServiceTiKV,
		},
	}
	kv.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceTiKV, kv)
	return kv, nil
}

func scaleInTiKVByTombstone(rt ControllerRuntime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
	kv, ok := inst.(*proc.TiKVInstance)
	if !ok {
		serviceID := ""
		if info := inst.Info(); info != nil {
			serviceID = info.Service.String()
		}
		return false, fmt.Errorf("unexpected instance type %T for service %s", inst, serviceID)
	}

	rt.ExpectExitPID(pid)
	c, err := pdClient(rt)
	if err != nil {
		return false, err
	}
	if err := c.DelStore(kv.StoreAddr(), nil); err != nil {
		return false, err
	}
	go watchAsyncScaleInStop(rt, 5*time.Second, func() (done bool, err error) {
		return c.IsTombStone(kv.StoreAddr())
	}, asyncScaleInStopEvent{
		serviceID:   proc.ServiceTiKV,
		inst:        kv,
		stopMessage: fmt.Sprintf("stop tombstone tikv %s", kv.StoreAddr()),
	})

	if w != nil {
		fmt.Fprintf(w, "requested scale-in %s (waiting for tombstone)\n", kv.StoreAddr())
	}
	return true, nil
}

func newTiKVWorkerInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	repoComponent := proc.ComponentTiKV
	if shOpt.Mode == proc.ModeNextGen {
		repoComponent = proc.ComponentTiKVWorker
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

	kvw := &proc.TiKVWorkerInstance{
		ShOpt: shOpt,
		Plan:  proc.TiKVWorkerPlan{PDAddrs: pdAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     proc.ResolveTiKVWorkerBinPath(params.Config.BinPath),
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, params.Config.Port, 19000, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: repoComponent,
			Service:         proc.ServiceTiKVWorker,
		},
	}
	kvw.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceTiKVWorker, kvw)
	return kvw, nil
}
