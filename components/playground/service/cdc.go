package service

import (
	"github.com/pingcap/tiup/components/playground/proc"
)

const (
	ticdcPortBase   = 8300
	tikvcdcPortBase = 8600
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiCDC,
		Catalog: Catalog{
			FlagPrefix:         "ticdc",
			AllowModifyNum:     true,
			AllowModifyHost:    true,
			AllowModifyPort:    true,
			DefaultPort:        ticdcPortBase,
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
		PlanInstance: func(_ BootContext, cfg proc.Config, alloc PortAllocator, plan *proc.ServicePlan) error {
			host := plan.Shared.Host

			portBase := ticdcPortBase
			if cfg.Port > 0 {
				portBase = cfg.Port
			}
			port, err := alloc(host, portBase)
			if err != nil {
				return err
			}

			plan.ComponentID = proc.ComponentCDC.String()
			plan.Shared.Port = port
			plan.Shared.StatusPort = port
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
			DefaultPort:        tikvcdcPortBase,
			DefaultNum:         func(_ BootContext) int { return 0 },
			IsEnabled:          func(_ BootContext) bool { return true },
			AllowScaleOut:      true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: newTiKVCDCInstance,
		PlanInstance: func(_ BootContext, _ proc.Config, alloc PortAllocator, plan *proc.ServicePlan) error {
			host := plan.Shared.Host

			port, err := alloc(host, tikvcdcPortBase)
			if err != nil {
				return err
			}

			plan.ComponentID = proc.ComponentTiKVCDC.String()
			plan.Shared.Port = port
			plan.Shared.StatusPort = port
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
	port := allocPort(params.Host, params.Config.Port, ticdcPortBase, shOpt.PortOffset)

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
			Host:            params.Host,
			Port:            port,
			StatusPort:      port,
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
	port := allocPort(params.Host, 0, tikvcdcPortBase, shOpt.PortOffset)

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
