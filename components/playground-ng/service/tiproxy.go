package service

import (
	"fmt"
	"io"
	"net"

	"github.com/pingcap/tiup/components/playground-ng/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiProxy,
		Catalog: Catalog{
			FlagPrefix:      "tiproxy",
			AllowModifyNum:  true,
			AllowModifyHost: true,
			AllowModifyPort: true,
			DefaultPort:     6000,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 6000, FromConfigPort: true},
				{Name: proc.PortNameStatusPort, Base: 3080},
			},
			AllowModifyConfig:  true,
			AllowModifyBinPath: true,
			AllowModifyTimeout: true,
			DefaultTimeout:     60,
			AllowModifyVersion: true,
			DefaultNum:         func(_ BootContext) int { return 0 },
			IsEnabled:          func(_ BootContext) bool { return true },
			AllowScaleOut:      true,
		},
		StartAfter: []proc.ServiceID{
			proc.ServicePD,
			proc.ServicePDAPI,
		},
		NewProc:      newTiProxyInstance,
		PostScaleOut: postScaleOutTiProxy,
		PlanInstance: func(_ BootContext, _ proc.Config, plan *proc.ServicePlan) error {
			plan.ComponentID = proc.ComponentTiProxy.String()
			return nil
		},
		FillServicePlans: func(_ BootContext, _ map[proc.ServiceID]proc.Config, byService map[proc.ServiceID][]*proc.ServicePlan, advertise func(listen string) string, plans []*proc.ServicePlan) error {
			pdBackendAddrs := plannedStatusAddrs(byService, advertise, proc.ServicePD, proc.ServicePDAPI)
			for _, sp := range plans {
				sp.TiProxy = &proc.TiProxyPlan{PDAddrs: pdBackendAddrs}
			}
			return nil
		},
	})
}

func newTiProxyInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	shared, err := allocPortsForNewProc(proc.ServiceTiProxy, params, shOpt.PortOffset)
	if err != nil {
		return nil, err
	}

	// TiProxy session certs are used by TiDB for session token signing. During
	// plan-based boot they are generated in Executor.PreRun; keep scale-out
	// behavior by ensuring they exist once the playground is already booted.
	if rt.Booted() {
		if err := proc.GenTiProxySessionCerts(rt.DataDir()); err != nil {
			return nil, err
		}
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

	proxy := &proc.TiProxyInstance{
		Plan: proc.TiProxyPlan{PDAddrs: pdAddrs},
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            shared.Host,
			Port:            shared.Port,
			StatusPort:      shared.StatusPort,
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiProxy,
			Service:         proc.ServiceTiProxy,
		},
	}
	proxy.UpTimeout = params.Config.UpTimeout
	rt.AddProc(proc.ServiceTiProxy, proxy)
	return proxy, nil
}

func postScaleOutTiProxy(w io.Writer, inst proc.Process) {
	if w == nil || inst == nil {
		return
	}
	proxy, ok := inst.(*proc.TiProxyInstance)
	if !ok {
		return
	}

	host, port, err := net.SplitHostPort(proxy.Addr())
	if err != nil {
		fmt.Fprintf(w, "TiProxy is up at %s\n", proxy.Addr())
		return
	}
	fmt.Fprintf(w, "Connect TiProxy: %s --host %s --port %s -u root\n", "mysql --comments", host, port)
}
