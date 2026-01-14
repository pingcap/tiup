package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground-ng/proc"
	pgservice "github.com/pingcap/tiup/components/playground-ng/service"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
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

type orderedStringIntMap map[string]int

func (m orderedStringIntMap) MarshalJSON() ([]byte, error) {
	return marshalOrderedStringMap(map[string]int(m))
}

type orderedStringConfigMap map[string]proc.Config

func (m orderedStringConfigMap) MarshalJSON() ([]byte, error) {
	return marshalOrderedStringMap(map[string]proc.Config(m))
}

func marshalOrderedStringMap[T any](m map[string]T) ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		encodedKey, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(encodedKey)
		b.WriteByte(':')
		encodedValue, err := json.Marshal(m[k])
		if err != nil {
			return nil, err
		}
		b.Write(encodedValue)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// MarshalJSON implements json.Marshaler and keeps the output stable for tests
// and dry-run diffing.
func (p BootPlan) MarshalJSON() ([]byte, error) {
	type bootPlanAlias BootPlan
	stable := struct {
		bootPlanAlias
		RequiredServices    orderedStringIntMap
		DebugServiceConfigs orderedStringConfigMap
	}{
		bootPlanAlias:       bootPlanAlias(p),
		RequiredServices:    orderedStringIntMap(p.RequiredServices),
		DebugServiceConfigs: orderedStringConfigMap(p.DebugServiceConfigs),
	}
	return json.Marshal(stable)
}

func writeDryRun(w io.Writer, plan BootPlan, format string) error {
	if w == nil {
		return fmt.Errorf("writer is nil")
	}
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		format = "text"
	}

	switch format {
	case "text":
		_, err := io.WriteString(w, renderDryRunText(w, plan))
		return err
	case "json":
		redacted := plan
		if strings.TrimSpace(redacted.Shared.CSE.AccessKey) != "" {
			redacted.Shared.CSE.AccessKey = "***"
		}
		if strings.TrimSpace(redacted.Shared.CSE.SecretKey) != "" {
			redacted.Shared.CSE.SecretKey = "***"
		}

		data, err := json.MarshalIndent(redacted, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	default:
		return fmt.Errorf("unknown --dry-run-output %q (expected text|json)", format)
	}
}

func renderDryRunText(out io.Writer, plan BootPlan) string {
	var b strings.Builder
	if out == nil {
		out = io.Discard
	}

	tokens := colorstr.DefaultTokens
	tokens.Disable = !tuiterm.Resolve(out).Color

	downloaded := make(map[string]struct{}, len(plan.Downloads))
	for _, d := range plan.Downloads {
		if d.ComponentID == "" || d.ResolvedVersion == "" {
			continue
		}
		downloaded[d.ComponentID+"@"+d.ResolvedVersion] = struct{}{}
	}

	var reusedComponents []string
	if len(plan.Services) > 0 {
		seen := make(map[string]struct{})
		for _, s := range plan.Services {
			if s.BinPath != "" || s.ComponentID == "" || s.ResolvedVersion == "" {
				continue
			}
			key := s.ComponentID + "@" + s.ResolvedVersion
			if _, ok := downloaded[key]; ok {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			reusedComponents = append(reusedComponents, key)
		}
		slices.Sort(reusedComponents)
	}

	if len(reusedComponents) > 0 {
		tokens.Fprintf(&b, "[light_magenta]==> [reset][bold]Existing Packages:[reset]\n")
		for _, c := range reusedComponents {
			componentID, resolved, ok := strings.Cut(c, "@")
			if !ok || componentID == "" || resolved == "" {
				continue
			}
			tokens.Fprintf(&b, "    %s[dim]@%s[reset]\n", componentID, resolved)
		}
	}

	if len(plan.Downloads) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		tokens.Fprintf(&b, "[light_magenta]==> [reset][bold]Download Packages:[reset]\n")
		for _, d := range plan.Downloads {
			if d.ComponentID == "" || d.ResolvedVersion == "" {
				continue
			}
			tokens.Fprintf(&b, "  [green]+[reset] %s[dim]@%s[reset]\n", d.ComponentID, d.ResolvedVersion)
		}
	}

	if len(plan.Services) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		tokens.Fprintf(&b, "[light_magenta]==> [reset][bold]Start Services:[reset]\n")
		for _, s := range plan.Services {
			if s.ServiceID == "" || s.Name == "" {
				continue
			}

			componentHint := ""
			if componentID := strings.TrimSpace(s.ComponentID); componentID != "" && componentID != strings.TrimSpace(s.ServiceID) {
				componentHint = componentID
			}

			tokens.Fprintf(&b, "  [green]+[reset] ")
			if componentHint != "" {
				tokens.Fprintf(&b, "%s/", componentHint)
			}
			if ver := s.ResolvedVersion; ver != "" {
				tokens.Fprintf(&b, "%s[dim]@%s[reset]", s.Name, ver)
			} else {
				tokens.Fprintf(&b, "%s", s.Name)
			}
			if binPath := strings.TrimSpace(s.BinPath); binPath != "" {
				tokens.Fprintf(&b, " [dim](use %s)[reset]", binPath)
			}
			b.WriteString("\n")

			host := strings.TrimSpace(s.Shared.Host)
			if host != "" && s.Shared.Port > 0 {
				addr := utils.JoinHostPort(host, s.Shared.Port)
				if s.Shared.StatusPort > 0 && s.Shared.StatusPort != s.Shared.Port {
					addr = fmt.Sprintf("%s,%d", addr, s.Shared.StatusPort)
				}
				tokens.Fprintf(&b, "    [dim]%s[reset]\n", addr)
			}

			if len(s.StartAfterServices) > 0 {
				tokens.Fprintf(&b, "    [dim]Start after: %s[reset]\n", strings.Join(s.StartAfterServices, ","))
			}
		}
	}

	return b.String()
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

func serviceVersionConstraint(serviceID proc.ServiceID, bootVersion string, options *BootOptions) string {
	constraint := bootVersion

	spec, ok := pgservice.SpecFor(serviceID)
	if !ok {
		return constraint
	}

	if spec.Catalog.AllowModifyVersion && options != nil {
		if cfg := options.Service(serviceID); cfg != nil && cfg.Version != "" {
			constraint = cfg.Version
		}
	}

	if bind := spec.Catalog.VersionBind; bind != nil {
		constraint = bind(constraint)
	}

	return constraint
}

func resolveVersionConstraint(serviceID proc.ServiceID, options *BootOptions) (string, error) {
	if options == nil {
		return "", nil
	}

	constraint := serviceVersionConstraint(serviceID, options.Version, options)
	if strings.TrimSpace(constraint) == "" {
		constraint = utils.LatestVersionAlias
	}
	return constraint, nil
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
	return buildBootPlanWithProcs(options, cfg, orderedServiceIDs, baseConfigs)
}

func buildBootPlanWithProcs(options *BootOptions, cfg bootPlannerConfig, orderedServiceIDs []proc.ServiceID, baseConfigs map[proc.ServiceID]proc.Config) (BootPlan, error) {
	if options == nil {
		return BootPlan{}, nil
	}
	cfg = cfg.normalize()

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
	allocPort := func(host string, base int) (int, error) {
		return pports.alloc(host, base, options.ShOpt.PortOffset)
	}

	for _, serviceID := range orderedServiceIDs {
		svcCfg := baseConfigs[serviceID]
		if serviceID == "" || svcCfg.Num <= 0 {
			continue
		}

		spec, ok := pgservice.SpecFor(serviceID)
		if !ok {
			return BootPlan{}, errors.Errorf("unknown service %s", serviceID)
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

		host := strings.TrimSpace(options.Host)
		if h := strings.TrimSpace(svcCfg.Host); h != "" {
			host = h
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
				BinPath:            svcCfg.BinPath,
				DebugConstraint:    constraint,
				ResolvedVersion:    constraint, // overwritten when resolved from repo
				Shared:             ServiceSharedPlan{Dir: dir, Host: host, ConfigPath: svcCfg.ConfigPath, UpTimeout: svcCfg.UpTimeout},
			}

			if spec.PlanInstance == nil {
				return BootPlan{}, errors.Errorf("missing planner rules for %s", serviceID)
			}
			if err := pgservice.FillPlannedPorts(allocPort, svcCfg, &sp, spec.Catalog.Ports); err != nil {
				return BootPlan{}, err
			}
			if err := spec.PlanInstance(options, svcCfg, &sp); err != nil {
				return BootPlan{}, err
			}
			if strings.TrimSpace(sp.ComponentID) == "" {
				return BootPlan{}, errors.Errorf("planned component id is empty for %s", serviceID)
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

	byService := make(map[proc.ServiceID][]*ServicePlan)
	for i := range servicePlans {
		sp := &servicePlans[i]
		serviceID := proc.ServiceID(strings.TrimSpace(sp.ServiceID))
		if serviceID == "" {
			continue
		}
		byService[serviceID] = append(byService[serviceID], sp)
	}

	plannedServiceIDs := make([]proc.ServiceID, 0, len(byService))
	for serviceID := range byService {
		plannedServiceIDs = append(plannedServiceIDs, serviceID)
	}
	slices.SortStableFunc(plannedServiceIDs, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	for _, serviceID := range plannedServiceIDs {
		spec, ok := pgservice.SpecFor(serviceID)
		if !ok {
			return BootPlan{}, errors.Errorf("unknown service %s", serviceID)
		}
		if spec.FillServicePlans == nil {
			continue
		}
		if err := spec.FillServicePlans(options, baseConfigs, byService, cfg.advertiseHost, byService[serviceID]); err != nil {
			return BootPlan{}, err
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
