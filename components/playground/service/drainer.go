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
		ServiceID: proc.ServiceDrainer,
		Catalog: Catalog{
			FlagPrefix:         "drainer",
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
		NewProc:     newDrainerInstance,
		ScaleInHook: scaleInDrainerByOffline,
	})
}

func newDrainerInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	port := allocPort(params.Host, 0, 8250, shOpt.PortOffset)
	drainer := &proc.Drainer{
		PDs: pds,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            port,
			StatusPort:      port,
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
