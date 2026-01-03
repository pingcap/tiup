package service

import (
	"fmt"
	"io"

	"github.com/pingcap/tiup/components/playground/proc"
)

func init() {
	for _, item := range []struct {
		serviceID  proc.ServiceID
		startAfter []proc.ServiceID
		scaleIn    ScaleInHookFunc
	}{
		{proc.ServicePD, nil, scaleInPDMember},
		{proc.ServicePDAPI, nil, scaleInPDMember},
		{proc.ServicePDTSO, []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI}, nil},
		{proc.ServicePDScheduling, []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI}, nil},
		{proc.ServicePDRouter, []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI}, nil},
		{proc.ServicePDResourceManager, []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI}, nil},
	} {
		registerPDService(item.serviceID, item.startAfter, item.scaleIn)
	}
}

func registerPDService(serviceID proc.ServiceID, startAfter []proc.ServiceID, scaleIn ScaleInHookFunc) {
	MustRegister(Spec{
		ServiceID: serviceID,
		NewProc: func(rt Runtime, params NewProcParams) (proc.Process, error) {
			return newPDInstance(rt, serviceID, params)
		},
		StartAfter:  startAfter,
		ScaleInHook: scaleIn,
	})
}

func scaleInPDMember(rt Runtime, _ io.Writer, inst proc.Process, _ int) (async bool, err error) {
	if rt == nil || inst == nil {
		return false, nil
	}
	info := inst.Info()
	if info == nil {
		return false, nil
	}
	switch info.Service {
	case proc.ServicePD, proc.ServicePDAPI:
	default:
		// Only PD members need explicit removal from the PD cluster.
		return false, nil
	}

	client, err := pdClient(rt)
	if err != nil {
		return false, err
	}
	return false, client.DelPD(info.Name(), nil)
}

func newPDInstance(rt Runtime, serviceID proc.ServiceID, params NewProcParams) (proc.Process, error) {
	members := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)

	kvIsSingleReplica := false
	if cfg, ok := rt.BootConfig(proc.ServiceTiKV); ok && cfg.Num == 1 {
		kvIsSingleReplica = true
	}

	shOpt := rt.SharedOptions()
	pd := &proc.PDInstance{
		ShOpt:             shOpt,
		PDs:               members,
		KVIsSingleReplica: kvIsSingleReplica,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, 0, 2380, shOpt.PortOffset),
			StatusPort:      allocPort(params.Host, params.Config.Port, 2379, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentPD,
			Service:         serviceID,
		},
	}
	pd.UpTimeout = params.Config.UpTimeout

	switch serviceID {
	case proc.ServicePD, proc.ServicePDAPI:
		if rt.Booted() {
			pd.Join(members)
			rt.AddProc(serviceID, pd)
		} else {
			rt.AddProc(serviceID, pd)
			all := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
			for _, member := range all {
				member.InitCluster(all)
			}
		}
	case proc.ServicePDTSO, proc.ServicePDScheduling, proc.ServicePDRouter, proc.ServicePDResourceManager:
		rt.AddProc(serviceID, pd)
	default:
		return nil, fmt.Errorf("unknown pd service %s", serviceID)
	}

	return pd, nil
}
