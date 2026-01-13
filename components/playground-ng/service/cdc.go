package service

import (
	"github.com/pingcap/tiup/components/playground-ng/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiCDC,
		Catalog: Catalog{
			FlagPrefix:      "ticdc",
			AllowModifyNum:  true,
			AllowModifyHost: true,
			AllowModifyPort: true,
			DefaultPort:     8300,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 8300, FromConfigPort: true},
				{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort},
			},
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			DefaultNum:         func(_ BootContext) int { return 0 },
			IsEnabled:          func(_ BootContext) bool { return true },
			AllowScaleOut:      true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: newTiCDCInstance,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentCDC.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)
			for _, sp := range plans {
				sp.TiCDC = &proc.TiCDCPlan{PDAddrs: pdBackendAddrs}
			}
			return nil
		},
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceTiKVCDC,
		Catalog: Catalog{
			FlagPrefix:         "kvcdc",
			AllowModifyNum:     true,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			AllowModifyVersion: true,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 8600},
				{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort},
			},
			DefaultNum:    func(_ BootContext) int { return 0 },
			IsEnabled:     func(_ BootContext) bool { return true },
			AllowScaleOut: true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: newTiKVCDCInstance,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentTiKVCDC.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)
			for _, sp := range plans {
				sp.TiKVCDC = &proc.TiKVCDCPlan{PDAddrs: pdBackendAddrs}
			}
			return nil
		},
	})
}

func newTiCDCInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(proc.ServiceTiCDC, params, shOpt.PortOffset)
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

	cdc := &proc.TiCDC{
		Plan: proc.TiCDCPlan{PDAddrs: pdAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentCDC,
			Service:         proc.ServiceTiCDC,
		},
	}
	cdc.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceTiCDC, cdc)
	return cdc, nil
}

func newTiKVCDCInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(proc.ServiceTiKVCDC, params, shOpt.PortOffset)
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

	kvcdc := &proc.TiKVCDCInstance{
		Plan: proc.TiKVCDCPlan{PDAddrs: pdAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiKVCDC,
			Service:         proc.ServiceTiKVCDC,
		},
	}
	kvcdc.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceTiKVCDC, kvcdc)
	return kvcdc, nil
}
