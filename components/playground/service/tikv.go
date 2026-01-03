package service

import (
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/cluster/api"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiKV,
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
			proc.ServicePDTSO,
		},
		NewProc:     newTiKVInstance,
		ScaleInHook: scaleInTiKVByTombstone,
	})
}

func newTiKVInstance(rt Runtime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	tsos := ProcsOf[*proc.PDInstance](rt, proc.ServicePDTSO)
	shOpt := rt.SharedOptions()
	kv := &proc.TiKVInstance{
		ShOpt: shOpt,
		PDs:   pds,
		TSOs:  tsos,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, params.Config.Port, 20160, shOpt.PortOffset),
			StatusPort:      allocPort(params.Host, 0, 20180, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiKV,
			Service:         proc.ServiceTiKV,
		},
	}
	kv.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceTiKV, kv)
	return kv, nil
}

func scaleInTiKVByTombstone(rt Runtime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
	kv, ok := inst.(*proc.TiKVInstance)
	if !ok {
		serviceID := ""
		if info := inst.Info(); info != nil {
			serviceID = info.Service.String()
		}
		return false, fmt.Errorf("unexpected instance type %T for service %s", inst, serviceID)
	}

	rt.ExpectExitPID(pid)
	c, err := pdClient(rt)
	if err != nil {
		return false, err
	}
	if err := c.DelStore(kv.StoreAddr(), nil); err != nil {
		return false, err
	}
	go watchTiKVTombstone(rt, c, kv)

	if w != nil {
		fmt.Fprintf(w, "requested scale-in %s (waiting for tombstone)\n", kv.StoreAddr())
	}
	return true, nil
}

type tiKVTombstoneEvent struct {
	inst *proc.TiKVInstance
}

func (e tiKVTombstoneEvent) Handle(rt Runtime) {
	if rt == nil || e.inst == nil {
		return
	}

	if !rt.RemoveProc(proc.ServiceTiKV, e.inst) {
		return
	}

	fmt.Fprintf(rt.TermWriter(), "stop tombstone tikv %s\n", e.inst.StoreAddr())
	pid := 0
	if proc := e.inst.Info().Proc; proc != nil {
		pid = proc.Pid()
	}
	rt.ExpectExitPID(pid)
	if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGQUIT); err != nil {
			fmt.Fprintln(rt.TermWriter(), err)
		}
	}

	rt.OnProcsChanged()
}

func watchTiKVTombstone(rt Runtime, c *api.PDClient, inst *proc.TiKVInstance) {
	if rt == nil || c == nil || inst == nil {
		return
	}
	pollUntil(rt, 5*time.Second, 0, func() (done bool, err error) {
		return c.IsTombStone(inst.StoreAddr())
	}, func() {
		rt.EmitEvent(tiKVTombstoneEvent{inst: inst})
	})
}
