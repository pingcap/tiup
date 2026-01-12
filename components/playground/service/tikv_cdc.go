package service

import "github.com/pingcap/tiup/components/playground/proc"

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiKVCDC,
		Catalog: Catalog{
			FlagPrefix:         "kvcdc",
			AllowModifyNum:     true,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			AllowModifyVersion: true,
			DefaultNum:         func(_ BootContext) int { return 0 },
			IsEnabled:          func(_ BootContext) bool { return true },
			AllowScaleOut:      true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: newTiKVCDCInstance,
	})
}

func newTiKVCDCInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	port := allocPort(params.Host, 0, 8600, shOpt.PortOffset)

	pdAddrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		if pd == nil {
			continue
		}
		if addr := pd.Addr(); addr != "" {
			pdAddrs = append(pdAddrs, addr)
		}
	}

	kvcdc := &proc.TiKVCDCInstance{
		Plan: proc.TiKVCDCPlan{PDAddrs: pdAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            port,
			StatusPort:      port,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiKVCDC,
			Service:         proc.ServiceTiKVCDC,
		},
	}
	kvcdc.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceTiKVCDC, kvcdc)
	return kvcdc, nil
}
