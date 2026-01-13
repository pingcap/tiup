package service

import (
	"fmt"
	"strings"

	"github.com/pingcap/tiup/components/playground-ng/proc"
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
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 9090, FromConfigPort: true},
				{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort},
			},
		},
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			shOpt := rt.SharedOptions()
			shared, err := allocPortsForNewProc(proc.ServicePrometheus, params, shOpt.PortOffset)
			if err != nil {
				return nil, err
			}
			prom := &proc.PrometheusInstance{
				ProcessInfo: proc.ProcessInfo{
					UserBinPath:     params.Config.BinPath,
					ID:              params.ID,
					Dir:             params.Dir,
					Host:            shared.Host,
					Port:            shared.Port,
					StatusPort:      shared.StatusPort,
					RepoComponentID: proc.ComponentPrometheus,
					Service:         proc.ServicePrometheus,
				},
			}
			rt.AddProc(proc.ServicePrometheus, prom)
			return prom, nil
		},
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentPrometheus.String()
			return nil
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
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 3000, FromConfigPort: true},
			},
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePrometheus,
		},
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			shOpt := rt.SharedOptions()
			shared, err := allocPortsForNewProc(proc.ServiceGrafana, params, shOpt.PortOffset)
			if err != nil {
				return nil, err
			}

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
					Host:            shared.Host,
					Port:            shared.Port,
					StatusPort:      shared.StatusPort,
					RepoComponentID: proc.ComponentGrafana,
					Service:         proc.ServiceGrafana,
				},
			}
			rt.AddProc(proc.ServiceGrafana, grafana)
			return grafana, nil
		},
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentGrafana.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			promURL := ""
			ps := byService[proc.ServicePrometheus]
			if len(ps) > 0 {
				host := advertise(ps[0].Shared.Host)
				promURL = fmt.Sprintf("http://%s", utils.JoinHostPort(host, ps[0].Shared.Port))
			}

			for _, sp := range plans {
				sp.Grafana = &proc.GrafanaPlan{PrometheusURL: promURL}
			}
			return nil
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
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 12020, FromConfigPort: true},
				{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort},
			},
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			shOpt := rt.SharedOptions()
			shared, err := allocPortsForNewProc(proc.ServiceNGMonitoring, params, shOpt.PortOffset)
			if err != nil {
				return nil, err
			}

			pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
			if len(pds) == 0 {
				return nil, fmt.Errorf("ng-monitoring requires PD")
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

			ngm := &proc.NGMonitoringInstance{
				Plan: proc.NGMonitoringPlan{PDAddrs: pdAddrs},
				ProcessInfo: proc.ProcessInfo{
					UserBinPath:     params.Config.BinPath,
					ID:              params.ID,
					Dir:             params.Dir,
					Host:            shared.Host,
					Port:            shared.Port,
					StatusPort:      shared.StatusPort,
					RepoComponentID: proc.ComponentPrometheus,
					Service:         proc.ServiceNGMonitoring,
				},
			}
			rt.AddProc(proc.ServiceNGMonitoring, ngm)
			return ngm, nil
		},
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			// NOTE: ng-monitoring-server is shipped alongside prometheus in TiUP.
			// Keep using prometheus as the repository identity for this service.
			plan.ComponentID = proc.ComponentPrometheus.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)
			for _, sp := range plans {
				sp.NGMonitoring = &proc.NGMonitoringPlan{PDAddrs: pdBackendAddrs}
			}
			return nil
		},
	})
}
