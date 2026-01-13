package service

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceDMMaster,
		Catalog: Catalog{
			FlagPrefix:      "dm-master",
			AllowModifyNum:  true,
			AllowModifyHost: true,
			AllowModifyPort: true,
			DefaultPort:     8261,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 8291},
				{Name: proc.PortNameStatusPort, Base: 8261, FromConfigPort: true},
			},
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			DefaultNum:         func(_ BootContext) int { return 0 },
			IsEnabled:          func(_ BootContext) bool { return true },
			AllowScaleOut:      true,
		},
		NewProc:     newDMMasterInstance,
		ScaleInHook: scaleInDMMaster,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentDMMaster.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, baseConfigs map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			members := plannedDMMembers(byService, advertise)

			waitReady := false
			if c, ok := baseConfigs[proc.ServiceDMWorker]; ok && c.Num > 0 {
				waitReady = true
			}

			for _, sp := range plans {
				sp.DMMaster = &proc.DMMasterPlan{InitialCluster: members, RequireReady: waitReady}
			}
			return nil
		},
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceDMWorker,
		Catalog: Catalog{
			FlagPrefix:      "dm-worker",
			AllowModifyNum:  true,
			AllowModifyHost: true,
			AllowModifyPort: true,
			DefaultPort:     8262,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 8262, FromConfigPort: true},
			},
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			DefaultNum:         func(_ BootContext) int { return 0 },
			IsEnabled:          func(_ BootContext) bool { return true },
			AllowScaleOut:      true,
		},
		NewProc:     newDMWorkerInstance,
		StartAfter:  []proc.ServiceID{proc.ServiceDMMaster},
		ScaleInHook: scaleInDMWorker,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentDMWorker.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			members := plannedDMMembers(byService, advertise)
			var addrs []string
			for _, m := range members {
				if m.MasterAddr != "" {
					addrs = append(addrs, m.MasterAddr)
				}
			}
			slices.Sort(addrs)
			addrs = slices.Compact(addrs)

			for _, sp := range plans {
				sp.DMWorker = &proc.DMWorkerPlan{MasterAddrs: addrs}
			}
			return nil
		},
	})
}

func plannedDMMembers(byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string) []proc.DMMemberPlan {
	if byService == nil {
		return nil
	}
	if advertise == nil {
		advertise = proc.AdvertiseHost
	}

	var members []proc.DMMemberPlan
	for _, sp := range byService[proc.ServiceDMMaster] {
		if sp.Name == "" || sp.Shared.Port <= 0 || sp.Shared.StatusPort <= 0 {
			continue
		}
		host := advertise(sp.Shared.Host)
		members = append(members, proc.DMMemberPlan{
			Name:       sp.Name,
			PeerAddr:   utils.JoinHostPort(host, sp.Shared.Port),
			MasterAddr: utils.JoinHostPort(host, sp.Shared.StatusPort),
		})
	}

	slices.SortFunc(members, func(a, b proc.DMMemberPlan) int { return strings.Compare(a.Name, b.Name) })
	return members
}

func newDMMasterInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	waitReady := false
	if cfg, ok := rt.BootConfig(proc.ServiceDMWorker); ok && cfg.Num > 0 {
		waitReady = true
	}

	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(proc.ServiceDMMaster, params, shOpt.PortOffset)
	if err != nil {
		return nil, err
	}
	master := &proc.DMMaster{
		Plan: proc.DMMasterPlan{
			RequireReady: waitReady,
		},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentDMMaster,
			Service:         proc.ServiceDMMaster,
		},
	}
	master.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceDMMaster, master)

	masters := ProcsOf[*proc.DMMaster](rt, proc.ServiceDMMaster)
	for _, m := range masters {
		var members []proc.DMMemberPlan
		for _, mm := range masters {
			if mm == nil {
				continue
			}
			info := mm.Info()
			if info == nil {
				continue
			}
			host := proc.AdvertiseHost(mm.Host)
			if mm.Port == 0 || mm.StatusPort == 0 {
				continue
			}
			members = append(members, proc.DMMemberPlan{
				Name:       info.Name(),
				PeerAddr:   utils.JoinHostPort(host, mm.Port),
				MasterAddr: utils.JoinHostPort(host, mm.StatusPort),
			})
		}
		m.Plan.InitialCluster = members
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

func newDMWorkerInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	masters := ProcsOf[*proc.DMMaster](rt, proc.ServiceDMMaster)
	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(proc.ServiceDMWorker, params, shOpt.PortOffset)
	if err != nil {
		return nil, err
	}

	masterAddrs := make([]string, 0, len(masters))
	for _, master := range masters {
		if master == nil {
			continue
		}
		if addr := master.Addr(); addr != "" {
			masterAddrs = append(masterAddrs, addr)
		}
	}

	worker := &proc.DMWorker{
		Plan: proc.DMWorkerPlan{MasterAddrs: masterAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentDMWorker,
			Service:         proc.ServiceDMWorker,
		},
	}
	worker.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceDMWorker, worker)
	return worker, nil
}

func scaleInDMWorker(rt ControllerRuntime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
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
