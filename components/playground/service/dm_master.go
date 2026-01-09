package service

import (
	"fmt"
	"io"

	"github.com/pingcap/tiup/components/playground/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceDMMaster,
		Catalog: Catalog{
			FlagPrefix:         "dm-master",
			AllowModifyNum:     true,
			AllowModifyHost:    true,
			AllowModifyPort:    true,
			DefaultPort:        8261,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			DefaultNum:         func(_ BootContext) int { return 0 },
			IsEnabled:          func(_ BootContext) bool { return true },
			AllowScaleOut:      true,
		},
		NewProc:     newDMMasterInstance,
		ScaleInHook: scaleInDMMaster,
	})
}

func newDMMasterInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	waitReady := false
	if cfg, ok := rt.BootConfig(proc.ServiceDMWorker); ok && cfg.Num > 0 {
		waitReady = true
	}

	shOpt := rt.SharedOptions()
	master := &proc.DMMaster{
		RequireReady: waitReady,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, 0, 8291, shOpt.PortOffset),
			StatusPort:      allocPort(params.Host, params.Config.Port, 8261, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentDMMaster,
			Service:         proc.ServiceDMMaster,
		},
	}
	master.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceDMMaster, master)

	masters := ProcsOf[*proc.DMMaster](rt, proc.ServiceDMMaster)
	for _, m := range masters {
		m.InitEndpoints = masters
	}
	return master, nil
}

func scaleInDMMaster(rt ControllerRuntime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
	if inst == nil {
		return false, nil
	}
	master, ok := inst.(*proc.DMMaster)
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
	name := master.Info().Name()
	if err := c.OfflineMaster(name, nil); err != nil {
		return false, err
	}
	if w != nil {
		fmt.Fprintf(w, "offlined %s\n", name)
	}
	return false, nil
}
