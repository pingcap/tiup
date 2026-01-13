package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pingcap/tiup/components/playground-ng/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServicePump,
		Catalog: Catalog{
			FlagPrefix:     "pump",
			AllowModifyNum: true,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 8249},
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
		NewProc:     newPumpInstance,
		ScaleInHook: scaleInPumpByOffline,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentPump.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)
			for _, sp := range plans {
				sp.Pump = &proc.PumpPlan{PDAddrs: pdBackendAddrs}
			}
			return nil
		},
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceDrainer,
		Catalog: Catalog{
			FlagPrefix:     "drainer",
			AllowModifyNum: true,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 8250},
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
		NewProc:     newDrainerInstance,
		ScaleInHook: scaleInDrainerByOffline,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentDrainer.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)
			for _, sp := range plans {
				sp.Drainer = &proc.DrainerPlan{PDAddrs: pdBackendAddrs}
			}
			return nil
		},
	})
}

func newPumpInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(proc.ServicePump, params, shOpt.PortOffset)
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

	pump := &proc.Pump{
		Plan: proc.PumpPlan{PDAddrs: pdAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentPump,
			Service:         proc.ServicePump,
		},
	}
	pump.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServicePump, pump)
	return pump, nil
}

func scaleInPumpByOffline(rt ControllerRuntime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
	pump, ok := inst.(*proc.Pump)
	if !ok {
		serviceID := ""
		if info := inst.Info(); info != nil {
			serviceID = info.Service.String()
		}
		return false, fmt.Errorf("unexpected instance type %T for service %s", inst, serviceID)
	}

	c, err := binlogClient(rt)
	if err != nil {
		return false, err
	}

	rt.ExpectExitPID(pid)
	ctxOffline, cancelOffline := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelOffline()
	if err := c.OfflinePump(ctxOffline, pump.Addr()); err != nil {
		return false, err
	}
	go watchAsyncScaleInStop(rt, 5*time.Second, func() (done bool, err error) {
		ctxProbe, cancelProbe := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelProbe()
		return c.IsPumpTombstone(ctxProbe, pump.Addr())
	}, asyncScaleInStopEvent{
		serviceID:   proc.ServicePump,
		inst:        pump,
		stopMessage: fmt.Sprintf("stop offline pump %s", pump.Addr()),
	})

	if w != nil {
		fmt.Fprintf(w, "requested scale-in %s (waiting for offline)\n", pump.Addr())
	}
	return true, nil
}

func newDrainerInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(proc.ServiceDrainer, params, shOpt.PortOffset)
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

	drainer := &proc.Drainer{
		Plan: proc.DrainerPlan{PDAddrs: pdAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentDrainer,
			Service:         proc.ServiceDrainer,
		},
	}
	drainer.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceDrainer, drainer)
	return drainer, nil
}

func scaleInDrainerByOffline(rt ControllerRuntime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
	drainer, ok := inst.(*proc.Drainer)
	if !ok {
		serviceID := ""
		if info := inst.Info(); info != nil {
			serviceID = info.Service.String()
		}
		return false, fmt.Errorf("unexpected instance type %T for service %s", inst, serviceID)
	}

	c, err := binlogClient(rt)
	if err != nil {
		return false, err
	}

	rt.ExpectExitPID(pid)
	ctxOffline, cancelOffline := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelOffline()
	if err := c.OfflineDrainer(ctxOffline, drainer.Addr()); err != nil {
		return false, err
	}
	go watchAsyncScaleInStop(rt, 5*time.Second, func() (done bool, err error) {
		ctxProbe, cancelProbe := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelProbe()
		return c.IsDrainerTombstone(ctxProbe, drainer.Addr())
	}, asyncScaleInStopEvent{
		serviceID:   proc.ServiceDrainer,
		inst:        drainer,
		stopMessage: fmt.Sprintf("stop offline drainer %s", drainer.Addr()),
	})

	if w != nil {
		fmt.Fprintf(w, "requested scale-in %s (waiting for offline)\n", drainer.Addr())
	}
	return true, nil
}
