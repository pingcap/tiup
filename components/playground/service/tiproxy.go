package service

import (
	"fmt"
	"io"
	"net"

	"github.com/pingcap/tiup/components/playground/proc"
)

func init() {
	MustRegister(Spec{
		ServiceID: proc.ServiceTiProxy,
		Catalog: Catalog{
			FlagPrefix:         "tiproxy",
			AllowModifyNum:     true,
			AllowModifyHost:    true,
			AllowModifyPort:    true,
			DefaultPort:        6000,
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
	})
}

func newTiProxyInstance(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
	if err := proc.GenTiProxySessionCerts(rt.DataDir()); err != nil {
		return nil, err
	}
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	shOpt := rt.SharedOptions()
	proxy := &proc.TiProxyInstance{
		PDs: pds,
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, params.Config.Port, 6000, shOpt.PortOffset),
			StatusPort:      allocPort(params.Host, 0, 3080, shOpt.PortOffset),
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
