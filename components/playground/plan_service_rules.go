package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

func planServicePorts(serviceID proc.ServiceID, svcCfg proc.Config, options *BootOptions, host string, pports *portPlanner, sp *ServicePlan) error {
	if options == nil {
		return errors.New("boot options is nil")
	}
	if sp == nil {
		return errors.New("service plan is nil")
	}

	switch serviceID {
	case proc.ServicePD, proc.ServicePDAPI, proc.ServicePDTSO, proc.ServicePDScheduling, proc.ServicePDRouter, proc.ServicePDResourceManager:
		peerPort, err := pports.alloc(host, 2380, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		statusPortBase := 2379
		if svcCfg.Port > 0 {
			statusPortBase = svcCfg.Port
		}
		statusPort, err := pports.alloc(host, statusPortBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = peerPort
		sp.Shared.StatusPort = statusPort
	case proc.ServiceTiKV:
		portBase := 20160
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		statusPort, err := pports.alloc(host, 20180, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = statusPort
	case proc.ServiceTiDB, proc.ServiceTiDBSystem:
		portBase := 4000
		if serviceID == proc.ServiceTiDBSystem {
			portBase = 3000
		}
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		statusPort, err := pports.alloc("0.0.0.0", 10080, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = statusPort
	case proc.ServiceTiKVWorker:
		// Keep existing "tikv-server -> tikv-worker" rewrite semantics.
		sp.BinPath = proc.ResolveTiKVWorkerBinPath(sp.BinPath)

		portBase := 19000
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
	case proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute:
		httpPort, err := pports.alloc(host, 8123, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		statusPort, err := pports.alloc(host, 8234, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		tcpPort, err := pports.alloc(host, 9100, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		servicePort, err := pports.alloc(host, 3930, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		proxyPort, err := pports.alloc(host, 20170, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		proxyStatusPort, err := pports.alloc(host, 20292, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = httpPort
		sp.Shared.StatusPort = statusPort
		sp.TiFlash = &proc.TiFlashPlan{
			ServicePort:     servicePort,
			TCPPort:         tcpPort,
			ProxyPort:       proxyPort,
			ProxyStatusPort: proxyStatusPort,
		}
	case proc.ServiceTiProxy:
		portBase := 6000
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		statusPort, err := pports.alloc(host, 3080, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = statusPort
	case proc.ServicePrometheus:
		portBase := 9090
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = port
	case proc.ServiceGrafana:
		portBase := 3000
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
	case proc.ServiceNGMonitoring:
		portBase := 12020
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = port
	case proc.ServiceTiCDC:
		portBase := 8300
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = port
	case proc.ServiceTiKVCDC:
		port, err := pports.alloc(host, 8600, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = port
	case proc.ServiceDMMaster:
		peerPort, err := pports.alloc(host, 8291, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		statusPortBase := 8261
		if svcCfg.Port > 0 {
			statusPortBase = svcCfg.Port
		}
		statusPort, err := pports.alloc(host, statusPortBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = peerPort
		sp.Shared.StatusPort = statusPort
	case proc.ServiceDMWorker:
		portBase := 8262
		if svcCfg.Port > 0 {
			portBase = svcCfg.Port
		}
		port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
	case proc.ServicePump:
		port, err := pports.alloc(host, 8249, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = port
	case proc.ServiceDrainer:
		port, err := pports.alloc(host, 8250, options.ShOpt.PortOffset)
		if err != nil {
			return err
		}
		sp.Shared.Port = port
		sp.Shared.StatusPort = port
	default:
		return errors.Errorf("missing port planning rules for %s", serviceID)
	}

	return nil
}

func fillServiceSpecificPlans(servicePlans []ServicePlan, baseConfigs map[proc.ServiceID]proc.Config, advertise func(listen string) string, options *BootOptions) {
	if options == nil {
		return
	}
	if advertise == nil {
		advertise = proc.AdvertiseHost
	}

	byService := make(map[proc.ServiceID][]*ServicePlan)
	for i := range servicePlans {
		sp := &servicePlans[i]
		serviceID := proc.ServiceID(strings.TrimSpace(sp.ServiceID))
		if serviceID == "" {
			continue
		}
		byService[serviceID] = append(byService[serviceID], sp)
	}

	pdBackendAddrs := func() []string {
		var out []string
		for _, sid := range []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI} {
			for _, sp := range byService[sid] {
				if sp == nil {
					continue
				}
				host := advertise(sp.Shared.Host)
				if sp.Shared.StatusPort > 0 {
					out = append(out, utils.JoinHostPort(host, sp.Shared.StatusPort))
				}
			}
		}
		slices.Sort(out)
		out = slices.Compact(out)
		return out
	}()

	tsoAddrs := func() []string {
		if options.ShOpt.PDMode != "ms" {
			return nil
		}
		var out []string
		for _, sp := range byService[proc.ServicePDTSO] {
			if sp == nil {
				continue
			}
			host := advertise(sp.Shared.Host)
			if sp.Shared.StatusPort > 0 {
				out = append(out, utils.JoinHostPort(host, sp.Shared.StatusPort))
			}
		}
		slices.Sort(out)
		out = slices.Compact(out)
		return out
	}()

	pdMembers := func() []proc.PDMemberPlan {
		var members []proc.PDMemberPlan
		for _, sid := range []proc.ServiceID{proc.ServicePD, proc.ServicePDAPI} {
			for _, sp := range byService[sid] {
				if sp == nil || sp.Name == "" {
					continue
				}
				host := advertise(sp.Shared.Host)
				members = append(members, proc.PDMemberPlan{
					Name:     sp.Name,
					PeerAddr: utils.JoinHostPort(host, sp.Shared.Port),
				})
			}
		}
		slices.SortFunc(members, func(a, b proc.PDMemberPlan) int { return strings.Compare(a.Name, b.Name) })
		return members
	}()

	dmMembers := func() []proc.DMMemberPlan {
		var members []proc.DMMemberPlan
		for _, sp := range byService[proc.ServiceDMMaster] {
			if sp == nil || sp.Name == "" {
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
	}()

	dmMasterAddrs := func() []string {
		var out []string
		for _, m := range dmMembers {
			if m.MasterAddr != "" {
				out = append(out, m.MasterAddr)
			}
		}
		slices.Sort(out)
		out = slices.Compact(out)
		return out
	}()

	promURL := func() string {
		ps := byService[proc.ServicePrometheus]
		if len(ps) == 0 || ps[0] == nil {
			return ""
		}
		host := advertise(ps[0].Shared.Host)
		if ps[0].Shared.Port <= 0 {
			return ""
		}
		return fmt.Sprintf("http://%s", utils.JoinHostPort(host, ps[0].Shared.Port))
	}()

	enableBinlog := false
	if c, ok := baseConfigs[proc.ServicePump]; ok && c.Num > 0 {
		enableBinlog = true
	}

	tikvWorkerURL := ""
	if ws := byService[proc.ServiceTiKVWorker]; len(ws) > 0 && ws[0] != nil {
		host := advertise(ws[0].Shared.Host)
		if ws[0].Shared.Port > 0 {
			tikvWorkerURL = fmt.Sprintf("http://%s", utils.JoinHostPort(host, ws[0].Shared.Port))
		}
	}

	for i := range servicePlans {
		sp := &servicePlans[i]
		if sp == nil {
			continue
		}
		serviceID := proc.ServiceID(strings.TrimSpace(sp.ServiceID))
		switch serviceID {
		case proc.ServicePD, proc.ServicePDAPI:
			kvSingle := false
			if c, ok := baseConfigs[proc.ServiceTiKV]; ok && c.Num == 1 {
				kvSingle = true
			}
			sp.PD = &proc.PDPlan{
				InitialCluster:    pdMembers,
				KVIsSingleReplica: kvSingle,
				BackendAddrs:      nil,
				JoinAddrs:         nil,
			}
		case proc.ServicePDTSO, proc.ServicePDScheduling, proc.ServicePDRouter, proc.ServicePDResourceManager:
			kvSingle := false
			if c, ok := baseConfigs[proc.ServiceTiKV]; ok && c.Num == 1 {
				kvSingle = true
			}
			sp.PD = &proc.PDPlan{
				BackendAddrs:      pdBackendAddrs,
				KVIsSingleReplica: kvSingle,
			}
		case proc.ServiceTiKV:
			sp.TiKV = &proc.TiKVPlan{
				PDAddrs:  pdBackendAddrs,
				TSOAddrs: tsoAddrs,
			}
		case proc.ServiceTiKVWorker:
			sp.TiKVWorker = &proc.TiKVWorkerPlan{PDAddrs: pdBackendAddrs}
		case proc.ServiceTiDB, proc.ServiceTiDBSystem:
			p := &proc.TiDBPlan{
				PDAddrs:       pdBackendAddrs,
				EnableBinlog:  enableBinlog,
				TiKVWorkerURL: "",
			}
			if serviceID == proc.ServiceTiDBSystem {
				p.TiKVWorkerURL = tikvWorkerURL
			}
			sp.TiDB = p
		case proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute:
			if sp.TiFlash == nil {
				sp.TiFlash = &proc.TiFlashPlan{}
			}
			sp.TiFlash.PDAddrs = pdBackendAddrs
		case proc.ServiceTiProxy:
			sp.TiProxy = &proc.TiProxyPlan{PDAddrs: pdBackendAddrs}
		case proc.ServiceNGMonitoring:
			sp.NGMonitoring = &proc.NGMonitoringPlan{PDAddrs: pdBackendAddrs}
		case proc.ServiceGrafana:
			sp.Grafana = &proc.GrafanaPlan{PrometheusURL: promURL}
		case proc.ServiceTiCDC:
			sp.TiCDC = &proc.TiCDCPlan{PDAddrs: pdBackendAddrs}
		case proc.ServiceTiKVCDC:
			sp.TiKVCDC = &proc.TiKVCDCPlan{PDAddrs: pdBackendAddrs}
		case proc.ServicePump:
			sp.Pump = &proc.PumpPlan{PDAddrs: pdBackendAddrs}
		case proc.ServiceDrainer:
			sp.Drainer = &proc.DrainerPlan{PDAddrs: pdBackendAddrs}
		case proc.ServiceDMMaster:
			waitReady := false
			if c, ok := baseConfigs[proc.ServiceDMWorker]; ok && c.Num > 0 {
				waitReady = true
			}
			sp.DMMaster = &proc.DMMasterPlan{InitialCluster: dmMembers, RequireReady: waitReady}
		case proc.ServiceDMWorker:
			sp.DMWorker = &proc.DMWorkerPlan{MasterAddrs: dmMasterAddrs}
		default:
		}
	}
}
