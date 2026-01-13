package service

import (
	"fmt"
	"io"
	"time"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

var tiflashPortSpecs = []PortSpec{
	{Name: proc.PortNamePort, Base: 8123},
	{Name: proc.PortNameStatusPort, Base: 8234},
	{Name: "tcp", Base: 9100},
	{Name: "service", Base: 3930},
	{Name: "proxy", Base: 20170},
	{Name: "proxyStatus", Base: 20292},
}

func planTiFlashInstance(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
	ports := plan.Shared.Ports
	for _, name := range []string{
		"service",
		"tcp",
		"proxy",
		"proxyStatus",
	} {
		if ports[name] <= 0 {
			return fmt.Errorf("missing planned port %q for tiflash", name)
		}
	}

	plan.ComponentID = proc.ComponentTiFlash.String()
	plan.TiFlash = &proc.TiFlashPlan{
		ServicePort:     ports["service"],
		TCPPort:         ports["tcp"],
		ProxyPort:       ports["proxy"],
		ProxyStatusPort: ports["proxyStatus"],
	}
	return nil
}

func fillTiFlashPlans(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
	pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)
	for _, sp := range plans {
		sp.TiFlash.PDAddrs = pdBackendAddrs
	}
	return nil
}

func init() {
	hasTiDB := func(ctx BootContext) bool {
		return ctx.ServiceConfigFor(proc.ServiceTiDB).Num > 0
	}

	startAfter := []proc.ServiceID{
		proc.ServicePD,
		proc.ServicePDAPI,
		proc.ServiceTiKV,
	}

	MustRegister(Spec{
		ServiceID: proc.ServiceTiFlash,
		Catalog: Catalog{
			FlagPrefix:         "tiflash",
			AllowModifyNum:     true,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			AllowModifyTimeout: true,
			DefaultTimeout:     120,
			Ports:              tiflashPortSpecs,
			DefaultNum: func(ctx BootContext) int {
				switch ctx.SharedOptions().Mode {
				case proc.ModeNormal, proc.ModeCSE, proc.ModeDisAgg:
					v := ctx.BootVersion()
					if utils.Version(v).IsValid() && !tidbver.TiFlashPlaygroundNewStartMode(v) {
						return 0
					}
					return 1
				default:
					return 0
				}
			},
			IsEnabled: func(ctx BootContext) bool {
				return ctx.SharedOptions().Mode == proc.ModeNormal && hasTiDB(ctx)
			},
			AllowScaleOut: true,
		},
		StartAfter: startAfter,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return newTiFlashInstance(rt, proc.ServiceTiFlash, params)
		},
		ScaleInHook:      scaleInTiFlashByTombstone,
		PlanInstance:     planTiFlashInstance,
		FillServicePlans: fillTiFlashPlans,
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceTiFlashWrite,
		Catalog: Catalog{
			FlagPrefix:         "tiflash.write",
			AllowModifyNum:     true,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			Ports:              tiflashPortSpecs,
			DefaultNum: func(ctx BootContext) int {
				switch ctx.SharedOptions().Mode {
				case proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg:
					return ctx.ServiceConfigFor(proc.ServiceTiFlash).Num
				default:
					return 0
				}
			},
			DefaultBinPathFrom:    proc.ServiceTiFlash,
			DefaultConfigPathFrom: proc.ServiceTiFlash,
			DefaultTimeoutFrom:    proc.ServiceTiFlash,
			IsEnabled: func(ctx BootContext) bool {
				if !hasTiDB(ctx) {
					return false
				}
				switch ctx.SharedOptions().Mode {
				case proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg:
					return true
				default:
					return false
				}
			},
			AllowScaleOut: true,
		},
		StartAfter: startAfter,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return newTiFlashInstance(rt, proc.ServiceTiFlashWrite, params)
		},
		ScaleInHook:      scaleInTiFlashByTombstone,
		PlanInstance:     planTiFlashInstance,
		FillServicePlans: fillTiFlashPlans,
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceTiFlashCompute,
		Catalog: Catalog{
			FlagPrefix:         "tiflash.compute",
			AllowModifyNum:     true,
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			Ports:              tiflashPortSpecs,
			DefaultNum: func(ctx BootContext) int {
				switch ctx.SharedOptions().Mode {
				case proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg:
					return ctx.ServiceConfigFor(proc.ServiceTiFlash).Num
				default:
					return 0
				}
			},
			DefaultBinPathFrom:    proc.ServiceTiFlash,
			DefaultConfigPathFrom: proc.ServiceTiFlash,
			DefaultTimeoutFrom:    proc.ServiceTiFlash,
			IsEnabled: func(ctx BootContext) bool {
				if !hasTiDB(ctx) {
					return false
				}
				switch ctx.SharedOptions().Mode {
				case proc.ModeCSE, proc.ModeNextGen, proc.ModeDisAgg:
					return true
				default:
					return false
				}
			},
			AllowScaleOut: true,
		},
		StartAfter: startAfter,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return newTiFlashInstance(rt, proc.ServiceTiFlashCompute, params)
		},
		ScaleInHook:      scaleInTiFlashByTombstone,
		PlanInstance:     planTiFlashInstance,
		FillServicePlans: fillTiFlashPlans,
	})
}

func scaleInTiFlashByTombstone(rt ControllerRuntime, w io.Writer, inst proc.Process, pid int) (async bool, err error) {
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
	serviceID := flash.Info().Service
	go watchAsyncScaleInStop(rt, 5*time.Second, func() (done bool, err error) {
		return c.IsTombStone(flash.StoreAddr())
	}, asyncScaleInStopEvent{
		serviceID:   serviceID,
		inst:        flash,
		stopMessage: fmt.Sprintf("stop tombstone tiflash %s", flash.StoreAddr()),
	})

	if w != nil {
		fmt.Fprintf(w, "requested scale-in %s (waiting for tombstone)\n", flash.StoreAddr())
	}
	return true, nil
}

func newTiFlashInstance(rt ControllerRuntime, serviceID proc.ServiceID, params NewProcParams) (proc.Process, error) {
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

	pdAddrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		if pd == nil {
			continue
		}
		if addr := pd.Addr(); addr != "" {
			pdAddrs = append(pdAddrs, addr)
		}
	}

	shared, err := allocPortsForNewProc(serviceID, params, shOpt.PortOffset)
	if err != nil {
		return nil, err
	}
	flash := &proc.TiFlashInstance{
		ShOpt: shOpt,
		Plan: proc.TiFlashPlan{
			PDAddrs:         pdAddrs,
			ServicePort:     shared.Ports["service"],
			TCPPort:         shared.Ports["tcp"],
			ProxyPort:       shared.Ports["proxy"],
			ProxyStatusPort: shared.Ports["proxyStatus"],
		},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiFlash,
			Service:         serviceID,
		},
	}
	flash.UpTimeout = params.Config.UpTimeout
	rt.AddProc(serviceID, flash)
	return flash, nil
}
