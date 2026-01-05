package service

import "github.com/pingcap/tiup/components/playground/proc"

func init() {
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

func newTiKVWorkerInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	repoComponent := proc.ComponentTiKV
	if shOpt.Mode == proc.ModeNextGen {
		repoComponent = proc.ComponentTiKVWorker
	}
	kvw := &proc.TiKVWorkerInstance{
		ShOpt: shOpt,
		PDs:   pds,
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
