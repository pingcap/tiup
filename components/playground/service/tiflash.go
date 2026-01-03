package service

import (
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/tidbver"
)

func init() {
	for _, serviceID := range []proc.ServiceID{
		proc.ServiceTiFlash,
		proc.ServiceTiFlashWrite,
		proc.ServiceTiFlashCompute,
	} {
		serviceID := serviceID
		MustRegister(Spec{
			ServiceID: serviceID,
			StartAfter: []proc.ServiceID{
				proc.ServicePD,
				proc.ServicePDAPI,
				proc.ServiceTiKV,
				proc.ServiceTiDBSystem,
				proc.ServiceTiDB,
			},
			NewProc: func(rt Runtime, params NewProcParams) (proc.Process, error) {
				return newTiFlashInstance(rt, serviceID, params)
			},
			ScaleInHook: scaleInTiFlashByTombstone,
		})
	}
}

func scaleInTiFlashByTombstone(rt Runtime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
	flash, ok := inst.(*proc.TiFlashInstance)
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
	if err := c.DelStore(flash.StoreAddr(), nil); err != nil {
		return false, err
	}
	go watchTiFlashTombstone(rt, c, flash)

	if w != nil {
		fmt.Fprintf(w, "requested scale-in %s (waiting for tombstone)\n", flash.StoreAddr())
	}
	return true, nil
}

type tiFlashTombstoneEvent struct {
	inst *proc.TiFlashInstance
}

func (e tiFlashTombstoneEvent) Handle(rt Runtime) {
	if rt == nil || e.inst == nil {
		return
	}

	serviceID := e.inst.Info().Service
	if !rt.RemoveProc(serviceID, e.inst) {
		return
	}

	fmt.Fprintf(rt.TermWriter(), "stop tombstone tiflash %s\n", e.inst.StoreAddr())
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

func watchTiFlashTombstone(rt Runtime, c *api.PDClient, inst *proc.TiFlashInstance) {
	if rt == nil || c == nil || inst == nil {
		return
	}
	pollUntil(rt, 5*time.Second, 0, func() (done bool, err error) {
		return c.IsTombStone(inst.StoreAddr())
	}, func() {
		rt.EmitEvent(tiFlashTombstoneEvent{inst: inst})
	})
}

func newTiFlashInstance(rt Runtime, serviceID proc.ServiceID, params NewProcParams) (proc.Process, error) {
	switch serviceID {
	case proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute:
	default:
		return nil, fmt.Errorf("unknown tiflash service %s", serviceID)
	}

	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	if (serviceID == proc.ServiceTiFlashWrite || serviceID == proc.ServiceTiFlashCompute) && shOpt.Mode != proc.ModeCSE && shOpt.Mode != proc.ModeDisAgg && shOpt.Mode != proc.ModeNextGen {
		return nil, fmt.Errorf("unsupported tiflash disagg service in mode %s", shOpt.Mode)
	}

	httpPort := 8123
	if !tidbver.TiFlashNotNeedHTTPPortConfig(params.Config.Version) {
		httpPort = allocPort(params.Host, 0, httpPort, shOpt.PortOffset)
	}
	flash := &proc.TiFlashInstance{
		ShOpt: shOpt,
		PDs:   pds,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            httpPort,
			StatusPort:      allocPort(params.Host, 0, 8234, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiFlash,
			Service:         serviceID,
		},
		TCPPort:         allocPort(params.Host, 0, 9100, shOpt.PortOffset),
		ServicePort:     allocPort(params.Host, 0, 3930, shOpt.PortOffset),
		ProxyPort:       allocPort(params.Host, 0, 20170, shOpt.PortOffset),
		ProxyStatusPort: allocPort(params.Host, 0, 20292, shOpt.PortOffset),
	}
	flash.UpTimeout = params.Config.UpTimeout
	rt.AddProc(serviceID, flash)
	return flash, nil
}
