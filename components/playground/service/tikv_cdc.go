package service

import "github.com/pingcap/tiup/components/playground/proc"

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiKVCDC,
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: newTiKVCDCInstance,
	})
}

func newTiKVCDCInstance(rt Runtime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	port := allocPort(params.Host, 0, 8600, shOpt.PortOffset)
	kvcdc := &proc.TiKVCDCInstance{
		PDs: pds,
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
