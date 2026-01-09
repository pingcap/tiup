package service

import (
	"fmt"
	"io"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

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
		ScaleInHook: scaleInTiFlashByTombstone,
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
		ScaleInHook: scaleInTiFlashByTombstone,
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
		ScaleInHook: scaleInTiFlashByTombstone,
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

	httpPort := allocPort(params.Host, 0, 8123, shOpt.PortOffset)
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
