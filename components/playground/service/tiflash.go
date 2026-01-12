package service

import (
	"fmt"
	"io"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	tiflashHTTPPortBase        = 8123
	tiflashStatusPortBase      = 8234
	tiflashTCPPortBase         = 9100
	tiflashServicePortBase     = 3930
	tiflashProxyPortBase       = 20170
	tiflashProxyStatusPortBase = 20292
)

func planTiFlashInstance(_ BootContext, _ proc.Config, alloc PortAllocator, plan *proc.ServicePlan) error {
	host := plan.Shared.Host

	httpPort, err := alloc(host, tiflashHTTPPortBase)
	if err != nil {
		return err
	}
	statusPort, err := alloc(host, tiflashStatusPortBase)
	if err != nil {
		return err
	}
	tcpPort, err := alloc(host, tiflashTCPPortBase)
	if err != nil {
		return err
	}
	servicePort, err := alloc(host, tiflashServicePortBase)
	if err != nil {
		return err
	}
	proxyPort, err := alloc(host, tiflashProxyPortBase)
	if err != nil {
		return err
	}
	proxyStatusPort, err := alloc(host, tiflashProxyStatusPortBase)
	if err != nil {
		return err
	}

	plan.ComponentID = proc.ComponentTiFlash.String()
	plan.Shared.Port = httpPort
	plan.Shared.StatusPort = statusPort
	plan.TiFlash = &proc.TiFlashPlan{
		ServicePort:     servicePort,
		TCPPort:         tcpPort,
		ProxyPort:       proxyPort,
		ProxyStatusPort: proxyStatusPort,
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

	httpPort := allocPort(params.Host, 0, tiflashHTTPPortBase, shOpt.PortOffset)
	statusPort := allocPort(params.Host, 0, tiflashStatusPortBase, shOpt.PortOffset)
	tcpPort := allocPort(params.Host, 0, tiflashTCPPortBase, shOpt.PortOffset)
	servicePort := allocPort(params.Host, 0, tiflashServicePortBase, shOpt.PortOffset)
	proxyPort := allocPort(params.Host, 0, tiflashProxyPortBase, shOpt.PortOffset)
	proxyStatusPort := allocPort(params.Host, 0, tiflashProxyStatusPortBase, shOpt.PortOffset)
	flash := &proc.TiFlashInstance{
		ShOpt: shOpt,
		Plan: proc.TiFlashPlan{
			PDAddrs:         pdAddrs,
			ServicePort:     servicePort,
			TCPPort:         tcpPort,
			ProxyPort:       proxyPort,
			ProxyStatusPort: proxyStatusPort,
		},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            httpPort,
			StatusPort:      statusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiFlash,
			Service:         serviceID,
		},
	}
	flash.UpTimeout = params.Config.UpTimeout
	rt.AddProc(serviceID, flash)
	return flash, nil
}
