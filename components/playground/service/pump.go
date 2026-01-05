package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServicePump,
		Catalog: Catalog{
			FlagPrefix:         "pump",
			AllowModifyNum:     true,
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
	})
}

func newPumpInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	port := allocPort(params.Host, 0, 8249, shOpt.PortOffset)
	pump := &proc.Pump{
		PDs: pds,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            port,
			StatusPort:      port,
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
