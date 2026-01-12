package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/utils"
)

// PortConflictPolicy controls how planner handles port allocation.
type PortConflictPolicy string

const (
	// PortConflictNone makes planner allocate ports deterministically without
	// probing the OS. It guarantees uniqueness within the generated plan, but it
	// does not guarantee the ports are actually free on the host machine.
	//
	// Uniqueness is scoped by host, except that "0.0.0.0" is treated as a
	// wildcard host that conflicts with all other hosts (matching typical OS
	// listen semantics).
	PortConflictNone PortConflictPolicy = "none"
	// PortConflictAllocFree makes planner allocate ports by probing the OS (the
	// current behavior of playground).
	PortConflictAllocFree PortConflictPolicy = "alloc_free"
)

// ComponentSource provides the minimal repository/filesystem surface needed to
// build and execute a BootPlan.
type ComponentSource interface {
	ResolveVersion(component, constraint string) (resolved string, err error)
	PlanInstall(serviceID proc.ServiceID, component, resolved string, forcePull bool) (*DownloadPlan, error)
	EnsureInstalled(component, resolved string) error
	BinaryPath(component, resolved string) (string, error)
}

// BootPlan is the deterministic, JSON-serializable plan produced by the
// playground planner and consumed by the executor.
//
// It is not persisted; it is intended for dry-run output, tests and immediate
// execution.
type BootPlan struct {
	// Global inputs (executor must not read flags/env again).
	DataDir     string
	BootVersion string
	Host        string

	// Shared options affecting both planning and execution.
	Shared proc.SharedOptions

	// UI-related knobs (do not affect core start semantics).
	Monitor     bool
	GrafanaPort int

	// RequiredServices is the minimum running instance count for critical
	// services. Controller uses it to trigger auto shutdown when critical
	// services exit unexpectedly.
	RequiredServices map[string]int

	// Downloads is the install action list for this plan (deduped and sorted).
	Downloads []DownloadPlan
	// Services is the per-instance start plan list (in stable order).
	Services []ServicePlan

	// Debug fields (not part of execution semantics).
	DebugServiceConfigs map[string]proc.Config
}

// DownloadPlan describes one component install/update action.
type DownloadPlan struct {
	ComponentID     string
	ResolvedVersion string

	DebugConstraint string
	DebugReason     string
	DebugSourceURL  string
	DebugInstallDir string
	DebugBinPath    string
}

// ServicePlan is the per-instance start plan.
type ServicePlan = proc.ServicePlan

// ServiceSharedPlan contains common, low-level per-instance inputs.
type ServiceSharedPlan = proc.ServiceSharedPlan

type bootPlannerConfig struct {
	dataDir            string
	portConflictPolicy PortConflictPolicy
	advertiseHost      func(listen string) string
	componentSource    ComponentSource
}

func (c bootPlannerConfig) normalize() bootPlannerConfig {
	if c.portConflictPolicy == "" {
		c.portConflictPolicy = PortConflictAllocFree
	}
	if c.advertiseHost == nil {
		c.advertiseHost = proc.AdvertiseHost
	}
	return c
}

type portPlanner struct {
	policy PortConflictPolicy
	used   map[string]map[int]struct{}
}

func newPortPlanner(policy PortConflictPolicy) *portPlanner {
	return &portPlanner{policy: policy}
}

func (p *portPlanner) alloc(host string, base, portOffset int) (int, error) {
	if p == nil {
		return 0, errors.New("port planner is nil")
	}
	if host == "" {
		return 0, errors.New("host is empty")
	}
	if base <= 0 {
		return 0, errors.New("base port is invalid")
	}

	switch p.policy {
	case PortConflictAllocFree:
		return utils.MustGetFreePort(host, base, portOffset), nil
	case PortConflictNone:
		want := base + portOffset
		if want <= 0 {
			return 0, errors.New("computed port is invalid")
		}
		if p.used == nil {
			p.used = make(map[string]map[int]struct{})
		}

		const wildcardHost = "0.0.0.0"
		if p.used[host] == nil {
			p.used[host] = make(map[int]struct{})
		}

		port := want
		for {
			if host == wildcardHost {
				conflict := false
				for _, usedPorts := range p.used {
					if _, ok := usedPorts[port]; ok {
						conflict = true
						break
					}
				}
				if !conflict {
					break
				}
				port++
				continue
			}

			if _, ok := p.used[host][port]; ok {
				port++
				continue
			}
			if usedPorts := p.used[wildcardHost]; usedPorts != nil {
				if _, ok := usedPorts[port]; ok {
					port++
					continue
				}
			}
			break
		}
		p.used[host][port] = struct{}{}
		return port, nil
	default:
		return 0, errors.Errorf("unknown port conflict policy %q", p.policy)
	}
}

func resolveVersionConstraint(serviceID proc.ServiceID, options *BootOptions) (string, error) {
	if options == nil {
		return "", nil
	}

	constraint := options.Version
	spec, ok := pgservice.SpecFor(serviceID)
	if ok && spec.Catalog.AllowModifyVersion {
		if cfg := options.Service(serviceID); cfg != nil && cfg.Version != "" {
			constraint = cfg.Version
		}
	}
	if ok && spec.Catalog.VersionBind != nil {
		constraint = spec.Catalog.VersionBind(constraint)
	}
	if strings.TrimSpace(constraint) == "" {
		constraint = utils.LatestVersionAlias
	}
	return constraint, nil
}

func repoComponentForService(serviceID proc.ServiceID, shOpt proc.SharedOptions) (proc.RepoComponentID, error) {
	switch serviceID {
	case proc.ServicePD,
		proc.ServicePDAPI,
		proc.ServicePDTSO,
		proc.ServicePDScheduling,
		proc.ServicePDRouter,
		proc.ServicePDResourceManager:
		return proc.ComponentPD, nil
	case proc.ServiceTiKV:
		return proc.ComponentTiKV, nil
	case proc.ServiceTiDB, proc.ServiceTiDBSystem:
		return proc.ComponentTiDB, nil
	case proc.ServiceTiKVWorker:
		if shOpt.Mode == proc.ModeNextGen {
			return proc.ComponentTiKVWorker, nil
		}
		return proc.ComponentTiKV, nil
	case proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute:
		return proc.ComponentTiFlash, nil
	case proc.ServiceTiProxy:
		return proc.ComponentTiProxy, nil
	case proc.ServicePrometheus:
		return proc.ComponentPrometheus, nil
	case proc.ServiceGrafana:
		return proc.ComponentGrafana, nil
	case proc.ServiceNGMonitoring:
		// NOTE: ng-monitoring-server is shipped alongside prometheus in TiUP.
		// Keep using prometheus as the repository identity for this service.
		return proc.ComponentPrometheus, nil
	case proc.ServiceTiCDC:
		return proc.ComponentCDC, nil
	case proc.ServiceTiKVCDC:
		return proc.ComponentTiKVCDC, nil
	case proc.ServiceDMMaster:
		return proc.ComponentDMMaster, nil
	case proc.ServiceDMWorker:
		return proc.ComponentDMWorker, nil
	case proc.ServicePump:
		return proc.ComponentPump, nil
	case proc.ServiceDrainer:
		return proc.ComponentDrainer, nil
	default:
		return "", errors.Errorf("unknown service %s", serviceID)
	}
}

// BuildBootPlan builds a deterministic BootPlan from BootOptions and the
// current local environment state (via ComponentSource).
func BuildBootPlan(options *BootOptions, cfg bootPlannerConfig) (BootPlan, error) {
	cfg = cfg.normalize()
	if options == nil {
		return BootPlan{}, nil
	}

	orderedServiceIDs, baseConfigs, err := planProcs(options)
	if err != nil {
		return BootPlan{}, err
	}
	if baseConfigs == nil {
		baseConfigs = make(map[proc.ServiceID]proc.Config)
	}

	required := make(map[string]int)
	for serviceID, c := range baseConfigs {
		if serviceID == "" || c.Num <= 0 {
			continue
		}
		spec, ok := pgservice.SpecFor(serviceID)
		if !ok || spec.Catalog.IsCritical == nil {
			continue
		}
		if spec.Catalog.IsCritical(options) {
			required[serviceID.String()] = 1
		}
	}

	debugConfigs := make(map[string]proc.Config, len(baseConfigs))
	for serviceID, c := range baseConfigs {
		if serviceID == "" {
			continue
		}
		debugConfigs[serviceID.String()] = c
	}

	pports := newPortPlanner(cfg.portConflictPolicy)

	type versionKey struct {
		component  string
		constraint string
	}

	versionCache := make(map[versionKey]string)
	downloadCache := make(map[string]*DownloadPlan) // key: component@resolved

	servicePlans := make([]ServicePlan, 0, len(orderedServiceIDs))

	for _, serviceID := range orderedServiceIDs {
		svcCfg := baseConfigs[serviceID]
		if serviceID == "" || svcCfg.Num <= 0 {
			continue
		}

		spec, ok := pgservice.SpecFor(serviceID)
		if !ok {
			return BootPlan{}, errors.Errorf("unknown service %s", serviceID)
		}

		componentID, err := repoComponentForService(serviceID, options.ShOpt)
		if err != nil {
			return BootPlan{}, err
		}

		constraint, err := resolveVersionConstraint(serviceID, options)
		if err != nil {
			return BootPlan{}, err
		}

		startAfter := make([]string, 0, len(spec.StartAfter))
		for _, dep := range spec.StartAfter {
			if dep == "" {
				continue
			}
			// Only include dependencies that are actually planned (num > 0).
			// Executor/runtime already treats missing deps as absent, so keeping
			// them here only adds noise to dry-run output without affecting
			// execution semantics.
			if baseConfigs[dep].Num <= 0 {
				continue
			}
			startAfter = append(startAfter, dep.String())
		}
		slices.Sort(startAfter)
		startAfter = slices.Compact(startAfter)

		host := options.Host
		if svcCfg.Host != "" {
			host = svcCfg.Host
		}

		for i := 0; i < svcCfg.Num; i++ {
			name := fmt.Sprintf("%s-%d", serviceID, i)
			dir := ""
			if cfg.dataDir != "" {
				dir = filepath.Join(cfg.dataDir, name)
			}

			sp := ServicePlan{
				Name:               name,
				ServiceID:          serviceID.String(),
				StartAfterServices: startAfter,
				ComponentID:        componentID.String(),
				BinPath:            svcCfg.BinPath,
				DebugConstraint:    constraint,
				ResolvedVersion:    constraint, // overwritten when resolved from repo
				Shared:             ServiceSharedPlan{Dir: dir, Host: host, ConfigPath: svcCfg.ConfigPath, UpTimeout: svcCfg.UpTimeout},
			}

			switch serviceID {
			case proc.ServicePD, proc.ServicePDAPI, proc.ServicePDTSO, proc.ServicePDScheduling, proc.ServicePDRouter, proc.ServicePDResourceManager:
				peerPort, err := pports.alloc(host, 2380, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				statusPortBase := 2379
				if svcCfg.Port > 0 {
					statusPortBase = svcCfg.Port
				}
				statusPort, err := pports.alloc(host, statusPortBase, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
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
					return BootPlan{}, err
				}
				statusPort, err := pports.alloc(host, 20180, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
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
					return BootPlan{}, err
				}
				statusPort, err := pports.alloc("0.0.0.0", 10080, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
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
					return BootPlan{}, err
				}
				sp.Shared.Port = port
			case proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute:
				httpPort, err := pports.alloc(host, 8123, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				statusPort, err := pports.alloc(host, 8234, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				tcpPort, err := pports.alloc(host, 9100, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				servicePort, err := pports.alloc(host, 3930, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				proxyPort, err := pports.alloc(host, 20170, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				proxyStatusPort, err := pports.alloc(host, 20292, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
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
					return BootPlan{}, err
				}
				statusPort, err := pports.alloc(host, 3080, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
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
					return BootPlan{}, err
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
					return BootPlan{}, err
				}
				sp.Shared.Port = port
			case proc.ServiceNGMonitoring:
				portBase := 12020
				if svcCfg.Port > 0 {
					portBase = svcCfg.Port
				}
				port, err := pports.alloc(host, portBase, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
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
					return BootPlan{}, err
				}
				sp.Shared.Port = port
				sp.Shared.StatusPort = port
			case proc.ServiceTiKVCDC:
				port, err := pports.alloc(host, 8600, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				sp.Shared.Port = port
				sp.Shared.StatusPort = port
			case proc.ServiceDMMaster:
				peerPort, err := pports.alloc(host, 8291, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				statusPortBase := 8261
				if svcCfg.Port > 0 {
					statusPortBase = svcCfg.Port
				}
				statusPort, err := pports.alloc(host, statusPortBase, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
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
					return BootPlan{}, err
				}
				sp.Shared.Port = port
			case proc.ServicePump:
				port, err := pports.alloc(host, 8249, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				sp.Shared.Port = port
				sp.Shared.StatusPort = port
			case proc.ServiceDrainer:
				port, err := pports.alloc(host, 8250, options.ShOpt.PortOffset)
				if err != nil {
					return BootPlan{}, err
				}
				sp.Shared.Port = port
				sp.Shared.StatusPort = port
			default:
				return BootPlan{}, errors.Errorf("missing port planning rules for %s", serviceID)
			}

			if cfg.componentSource != nil && sp.BinPath == "" {
				cacheKey := versionKey{component: sp.ComponentID, constraint: constraint}
				resolved := versionCache[cacheKey]
				if resolved == "" {
					r, err := cfg.componentSource.ResolveVersion(sp.ComponentID, constraint)
					if err != nil {
						return BootPlan{}, err
					}
					resolved = r
					versionCache[cacheKey] = resolved
				}
				sp.ResolvedVersion = resolved

				dp, err := cfg.componentSource.PlanInstall(serviceID, sp.ComponentID, resolved, options.ShOpt.ForcePull)
				if err != nil {
					return BootPlan{}, err
				}
				if dp != nil {
					key := sp.ComponentID + "@" + resolved
					if _, ok := downloadCache[key]; !ok {
						downloadCache[key] = dp
					}
				}
			}

			servicePlans = append(servicePlans, sp)
		}
	}

	// Build serviceID -> plans mapping for dependency derivation.
	byService := make(map[string][]*ServicePlan)
	for i := range servicePlans {
		sp := &servicePlans[i]
		if sp.ServiceID == "" {
			continue
		}
		byService[sp.ServiceID] = append(byService[sp.ServiceID], sp)
	}

	advertise := cfg.advertiseHost
	pdBackendAddrs := func() []string {
		var out []string
		for _, sid := range []string{proc.ServicePD.String(), proc.ServicePDAPI.String()} {
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
		for _, sp := range byService[proc.ServicePDTSO.String()] {
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
		for _, sid := range []string{proc.ServicePD.String(), proc.ServicePDAPI.String()} {
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
		for _, sp := range byService[proc.ServiceDMMaster.String()] {
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
		ps := byService[proc.ServicePrometheus.String()]
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
	if ws := byService[proc.ServiceTiKVWorker.String()]; len(ws) > 0 && ws[0] != nil {
		host := advertise(ws[0].Shared.Host)
		if ws[0].Shared.Port > 0 {
			tikvWorkerURL = fmt.Sprintf("http://%s", utils.JoinHostPort(host, ws[0].Shared.Port))
		}
	}

	// Fill service-specific plans.
	for i := range servicePlans {
		sp := &servicePlans[i]
		if sp == nil {
			continue
		}
		serviceID := proc.ServiceID(sp.ServiceID)
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

	// Finalize downloads list: stable order, de-duped by component@resolved.
	downloads := make([]DownloadPlan, 0, len(downloadCache))
	for _, dp := range downloadCache {
		if dp == nil || dp.ComponentID == "" || dp.ResolvedVersion == "" {
			continue
		}
		downloads = append(downloads, *dp)
	}
	slices.SortFunc(downloads, func(a, b DownloadPlan) int {
		if c := strings.Compare(a.ComponentID, b.ComponentID); c != 0 {
			return c
		}
		return strings.Compare(a.ResolvedVersion, b.ResolvedVersion)
	})

	bootVer := strings.TrimSpace(options.Version)
	if bootVer == "" {
		bootVer = utils.LatestVersionAlias
	}

	return BootPlan{
		DataDir:             cfg.dataDir,
		BootVersion:         bootVer,
		Host:                options.Host,
		Shared:              options.ShOpt,
		Monitor:             options.Monitor,
		GrafanaPort:         options.GrafanaPort,
		RequiredServices:    required,
		Downloads:           downloads,
		Services:            servicePlans,
		DebugServiceConfigs: debugConfigs,
	}, nil
}
