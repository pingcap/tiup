package service

import (
	"context"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/cluster/api"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceDrainer,
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc:     newDrainerInstance,
		ScaleInHook: scaleInDrainerByOffline,
	})
}

func newDrainerInstance(rt Runtime, params NewProcParams) (proc.Process, error) {
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

func scaleInDrainerByOffline(rt Runtime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
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
	go watchDrainerTombstone(rt, c, drainer)

	if w != nil {
		fmt.Fprintf(w, "requested scale-in %s (waiting for offline)\n", drainer.Addr())
	}
	return true, nil
}

type drainerTombstoneEvent struct {
	inst *proc.Drainer
}

func (e drainerTombstoneEvent) Handle(rt Runtime) {
	if rt == nil || e.inst == nil {
		return
	}

	if !rt.RemoveProc(proc.ServiceDrainer, e.inst) {
		return
	}

	fmt.Fprintf(rt.TermWriter(), "stop offline drainer %s\n", e.inst.Addr())
	pid := 0
	if info := e.inst.Info(); info != nil && info.Proc != nil {
		pid = info.Proc.Pid()
	}
	rt.ExpectExitPID(pid)
	if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGQUIT); err != nil {
			fmt.Fprintln(rt.TermWriter(), err)
		}
	}

	rt.OnProcsChanged()
}

func watchDrainerTombstone(rt Runtime, c *api.BinlogClient, inst *proc.Drainer) {
	if rt == nil || c == nil || inst == nil {
		return
	}
	pollUntil(rt, 5*time.Second, 0, func() (done bool, err error) {
		ctxProbe, cancelProbe := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelProbe()
		return c.IsDrainerTombstone(ctxProbe, inst.Addr())
	}, func() {
		rt.EmitEvent(drainerTombstoneEvent{inst: inst})
	})
}
