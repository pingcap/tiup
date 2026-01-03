package service

import "github.com/pingcap/tiup/components/playground/proc"

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiCDC,
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: newTiCDCInstance,
	})
}

func newTiCDCInstance(rt Runtime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	port := allocPort(params.Host, params.Config.Port, 8300, shOpt.PortOffset)
	cdc := &proc.TiCDC{
		PDs: pds,
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
