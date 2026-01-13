package service

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

var pdPortSpecs = []PortSpec{
	{Name: proc.PortNamePort, Base: 2380},
	{Name: proc.PortNameStatusPort, Base: 2379, FromConfigPort: true},
}

var pdMicroservicePortSpecs = []PortSpec{
	{Name: proc.PortNameStatusPort, Base: 2379},
	{Name: proc.PortNamePort, AliasOf: proc.PortNameStatusPort},
}

func init() {
	for _, item := range []struct {
		serviceID  proc.ServiceID
		startAfter []proc.ServiceID
		scaleIn    ScaleInHookFunc
		catalog    Catalog
	}{
		{
			serviceID: proc.ServicePD,
			scaleIn:   scaleInPDMember,
			catalog: Catalog{
				FlagPrefix:         "pd",
				AllowModifyNum:     true,
				AllowModifyHost:    true,
				AllowModifyPort:    true,
				DefaultPort:        2379,
				Ports:              pdPortSpecs,
				AllowModifyConfig:  true,
				AllowModifyBinPath: true,
				DefaultNum:         func(_ BootContext) int { return 1 },
				IsEnabled:          func(ctx BootContext) bool { return ctx.SharedOptions().PDMode != "ms" },
				IsCritical:         func(ctx BootContext) bool { return ctx.SharedOptions().PDMode != "ms" },
				AllowScaleOut:      true,
			},
		},
		{
			serviceID: proc.ServicePDAPI,
			scaleIn:   scaleInPDMember,
			catalog: Catalog{
				FlagPrefix:         "pd.api",
				AllowModifyNum:     true,
				AllowModifyHost:    true,
				AllowModifyPort:    true,
				DefaultPort:        2379,
				Ports:              pdPortSpecs,
				AllowModifyConfig:  true,
				AllowModifyBinPath: true,
				DefaultNum: func(ctx BootContext) int {
					if ctx.SharedOptions().PDMode != "ms" {
						return 0
					}
					// Default to the same count as `--pd`, so users can configure the
					// PD microservices cluster size with one flag and override it with
					// `--pd.api` when needed.
					return ctx.ServiceConfigFor(proc.ServicePD).Num
				},
				DefaultBinPathFrom:    proc.ServicePD,
				DefaultConfigPathFrom: proc.ServicePD,
				DefaultHostFrom:       proc.ServicePD,
				DefaultPortFrom:       proc.ServicePD,
				IsEnabled:             func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				IsCritical:            func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				AllowScaleOut:         true,
			},
		},
		{
			serviceID:  proc.ServicePDTSO,
			startAfter: []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI},
			catalog: Catalog{
				FlagPrefix:         "pd.tso",
				AllowModifyNum:     true,
				AllowModifyHost:    true,
				Ports:              pdMicroservicePortSpecs,
				AllowModifyConfig:  true,
				AllowModifyBinPath: true,
				DefaultNum: func(ctx BootContext) int {
					if ctx.SharedOptions().PDMode == "ms" {
						return 1
					}
					return 0
				},
				DefaultBinPathFrom:    proc.ServicePD,
				DefaultConfigPathFrom: proc.ServicePD,
				IsEnabled:             func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				IsCritical:            func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				AllowScaleOut:         true,
			},
		},
		{
			serviceID:  proc.ServicePDScheduling,
			startAfter: []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI},
			catalog: Catalog{
				FlagPrefix:         "pd.sched",
				AllowModifyNum:     true,
				AllowModifyHost:    true,
				Ports:              pdMicroservicePortSpecs,
				AllowModifyConfig:  true,
				AllowModifyBinPath: true,
				DefaultNum: func(ctx BootContext) int {
					if ctx.SharedOptions().PDMode == "ms" {
						return 1
					}
					return 0
				},
				DefaultBinPathFrom:    proc.ServicePD,
				DefaultConfigPathFrom: proc.ServicePD,
				IsEnabled:             func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				IsCritical:            func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				AllowScaleOut:         true,
			},
		},
		{
			serviceID:  proc.ServicePDRouter,
			startAfter: []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI},
			catalog: Catalog{
				FlagPrefix:            "pd.router",
				AllowModifyNum:        true,
				AllowModifyHost:       true,
				Ports:                 pdMicroservicePortSpecs,
				AllowModifyConfig:     true,
				AllowModifyBinPath:    true,
				DefaultNum:            func(_ BootContext) int { return 0 },
				DefaultBinPathFrom:    proc.ServicePD,
				DefaultConfigPathFrom: proc.ServicePD,
				IsEnabled:             func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				AllowScaleOut:         true,
			},
		},
		{
			serviceID:  proc.ServicePDResourceManager,
			startAfter: []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI},
			catalog: Catalog{
				FlagPrefix:            "pd.res",
				AllowModifyNum:        true,
				AllowModifyHost:       true,
				Ports:                 pdMicroservicePortSpecs,
				AllowModifyConfig:     true,
				AllowModifyBinPath:    true,
				DefaultNum:            func(_ BootContext) int { return 0 },
				DefaultBinPathFrom:    proc.ServicePD,
				DefaultConfigPathFrom: proc.ServicePD,
				IsEnabled:             func(ctx BootContext) bool { return ctx.SharedOptions().PDMode == "ms" },
				AllowScaleOut:         true,
			},
		},
	} {
		registerPDService(item.serviceID, item.startAfter, item.scaleIn, item.catalog)
	}
}

func registerPDService(serviceID proc.ServiceID, startAfter []proc.ServiceID, scaleIn ScaleInHookFunc, catalog Catalog) {
	MustRegister(Spec{
		ServiceID: serviceID,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return newPDInstance(rt, serviceID, params)
		},
		Catalog:     catalog,
		StartAfter:  startAfter,
		ScaleInHook: scaleIn,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentPD.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, baseConfigs map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			var members []proc.PDMemberPlan
			for _, sid := range []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI} {
				for _, sp := range byService[sid] {
					host := advertise(sp.Shared.Host)
					members = append(members, proc.PDMemberPlan{
						Name:     sp.Name,
						PeerAddr: utils.JoinHostPort(host, sp.Shared.Port),
					})
				}
			}
			slices.SortFunc(members, func(a, b proc.PDMemberPlan) int { return strings.Compare(a.Name, b.Name) })

			backendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)

			kvSingle := false
			if c, ok := baseConfigs[proc.ServiceTiKV]; ok && c.Num == 1 {
				kvSingle = true
			}

			for _, sp := range plans {
				switch serviceID {
				case proc.ServicePD, proc.ServicePDAPI:
					sp.PD = &proc.PDPlan{
						InitialCluster:    members,
						KVIsSingleReplica: kvSingle,
					}
				case proc.ServicePDTSO, proc.ServicePDScheduling, proc.ServicePDRouter, proc.ServicePDResourceManager:
					sp.PD = &proc.PDPlan{
						BackendAddrs:      backendAddrs,
						KVIsSingleReplica: kvSingle,
					}
				default:
					return fmt.Errorf("unknown pd service %s", serviceID)
				}
			}
			return nil
		},
	})
}

func scaleInPDMember(rt ControllerRuntime, _ io.Writer, inst proc.Process, _ int) (async bool, err error) {
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

func newPDInstance(rt ControllerRuntime, serviceID proc.ServiceID, params NewProcParams) (proc.Process, error) {
	members := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)

	kvIsSingleReplica := false
	if cfg, ok := rt.BootConfig(proc.ServiceTiKV); ok && cfg.Num == 1 {
		kvIsSingleReplica = true
	}

	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(serviceID, params, shOpt.PortOffset)
	if err != nil {
		return nil, err
	}
	pd := &proc.PDInstance{
		ShOpt: shOpt,
		Plan: proc.PDPlan{
			KVIsSingleReplica: kvIsSingleReplica,
		},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
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
		for _, m := range members {
			if m == nil {
				continue
			}
			host := proc.AdvertiseHost(m.Host)
			pd.Plan.BackendAddrs = append(pd.Plan.BackendAddrs, utils.JoinHostPort(host, m.StatusPort))
		}
		rt.AddProc(serviceID, pd)
	default:
		return nil, fmt.Errorf("unknown pd service %s", serviceID)
	}

	return pd, nil
}
