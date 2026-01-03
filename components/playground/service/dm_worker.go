package service

import (
	"fmt"
	"io"

	"github.com/pingcap/tiup/components/playground/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID:   proc.ServiceDMWorker,
		NewProc:     newDMWorkerInstance,
		StartAfter:  []proc.ServiceID{proc.ServiceDMMaster},
		ScaleInHook: scaleInDMWorker,
	})
}

func newDMWorkerInstance(rt Runtime, params NewProcParams) (proc.Process, error) {
	masters := ProcsOf[*proc.DMMaster](rt, proc.ServiceDMMaster)
	shOpt := rt.SharedOptions()
	worker := &proc.DMWorker{
		Masters: masters,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, params.Config.Port, 8262, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentDMWorker,
			Service:         proc.ServiceDMWorker,
		},
	}
	worker.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceDMWorker, worker)
	return worker, nil
}

func scaleInDMWorker(rt Runtime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
	if inst == nil {
		return false, nil
	}
	worker, ok := inst.(*proc.DMWorker)
	if !ok {
		serviceID := ""
		if info := inst.Info(); info != nil {
			serviceID = info.Service.String()
		}
		return false, fmt.Errorf("unexpected instance type %T for service %s", inst, serviceID)
	}

	c, err := dmMasterClient(rt)
	if err != nil {
		return false, err
	}

	rt.ExpectExitPID(pid)
	name := worker.Info().Name()
	if err := c.OfflineWorker(name, nil); err != nil {
		return false, err
	}
	if w != nil {
		fmt.Fprintf(w, "offlined %s\n", name)
	}
	return false, nil
}
