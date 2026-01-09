package service

import (
	"fmt"
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

func stripNextGenVersionSuffix(version string) string {
	return strings.TrimSuffix(version, "-"+utils.NextgenVersionAlias)
}

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServicePrometheus,
		Catalog: Catalog{
			IsEnabled:      func(ctx BootContext) bool { return ctx.MonitorEnabled() },
			PlanConfig:     func(_ BootContext) proc.Config { return proc.Config{Num: 1} },
			VersionBind:    stripNextGenVersionSuffix,
			HideInProgress: true,
		},
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			shOpt := rt.SharedOptions()
			port := allocPort(params.Host, params.Config.Port, 9090, shOpt.PortOffset)
			prom := &proc.PrometheusInstance{
				ProcessInfo: proc.ProcessInfo{
					UserBinPath:     params.Config.BinPath,
					ID:              params.ID,
					Dir:             params.Dir,
					Host:            params.Host,
					Port:            port,
					StatusPort:      port,
					RepoComponentID: proc.ComponentPrometheus,
					Service:         proc.ServicePrometheus,
				},
			}
			rt.AddProc(proc.ServicePrometheus, prom)
			return prom, nil
		},
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceGrafana,
		Catalog: Catalog{
			IsEnabled: func(ctx BootContext) bool { return ctx.MonitorEnabled() },
			PlanConfig: func(ctx BootContext) proc.Config {
				port := ctx.GrafanaPortOverride()
				return proc.Config{Num: 1, Port: port}
			},
			VersionBind:    stripNextGenVersionSuffix,
			HideInProgress: true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePrometheus,
		},
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			shOpt := rt.SharedOptions()
			port := allocPort(params.Host, params.Config.Port, 3000, shOpt.PortOffset)

			promURL := ""
			if ps := ProcsOf[*proc.PrometheusInstance](rt, proc.ServicePrometheus); len(ps) > 0 && ps[0] != nil {
				promURL = fmt.Sprintf("http://%s", utils.JoinHostPort(proc.AdvertiseHost(ps[0].Host), ps[0].Port))
			}

			grafana := &proc.GrafanaInstance{
				PrometheusURL: promURL,
				ProcessInfo: proc.ProcessInfo{
					UserBinPath:     params.Config.BinPath,
					ID:              params.ID,
					Dir:             params.Dir,
					Host:            params.Host,
					Port:            port,
					RepoComponentID: proc.ComponentGrafana,
					Service:         proc.ServiceGrafana,
				},
			}
			rt.AddProc(proc.ServiceGrafana, grafana)
			return grafana, nil
		},
	})

	MustRegister(Spec{
		ServiceID: proc.ServiceNGMonitoring,
		Catalog: Catalog{
			IsEnabled: func(ctx BootContext) bool {
				if !ctx.MonitorEnabled() {
					return false
				}
				// ng-monitoring-server is only available in newer releases. Skip it on
				// versions known to not include it.
				baseVersion := stripNextGenVersionSuffix(ctx.BootVersion())
				if !utils.Version(baseVersion).IsValid() {
					return true
				}
				return semver.Compare(baseVersion, "v5.3.0") >= 0
			},
			PlanConfig:     func(_ BootContext) proc.Config { return proc.Config{Num: 1} },
			VersionBind:    stripNextGenVersionSuffix,
			HideInProgress: true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			shOpt := rt.SharedOptions()
			port := allocPort(params.Host, params.Config.Port, 12020, shOpt.PortOffset)

			pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
			if len(pds) == 0 {
				return nil, fmt.Errorf("ng-monitoring requires PD")
			}

			ngm := &proc.NGMonitoringInstance{
				PDs: pds,
				ProcessInfo: proc.ProcessInfo{
					UserBinPath:     params.Config.BinPath,
					ID:              params.ID,
					Dir:             params.Dir,
					Host:            params.Host,
					Port:            port,
					StatusPort:      port,
					RepoComponentID: proc.ComponentPrometheus,
					Service:         proc.ServiceNGMonitoring,
				},
			}
			rt.AddProc(proc.ServiceNGMonitoring, ngm)
			return ngm, nil
		},
	})
}
