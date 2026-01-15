package main

import (
	"context"
	stdErrors "errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground-ng/proc"
	pgservice "github.com/pingcap/tiup/components/playground-ng/service"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

// BootOptions is the topology and options used to start a playground cluster.
//
// Per-service options are stored in Services to avoid the "add a service, update
// N different field lists" failure mode.
type BootOptions struct {
	ShOpt       proc.SharedOptions `yaml:"shared_opt"`
	Version     string             `yaml:"version"`
	Host        string             `yaml:"host"`
	Monitor     bool               `yaml:"monitor"`
	GrafanaPort int                `yaml:"grafana_port"`

	Services map[proc.ServiceID]*proc.Config `yaml:"services,omitempty"`
}

// Service returns the mutable per-service config, allocating it on demand.
func (o *BootOptions) Service(serviceID proc.ServiceID) *proc.Config {
	if o == nil || serviceID == "" {
		return nil
	}
	if o.Services == nil {
		o.Services = make(map[proc.ServiceID]*proc.Config)
	}
	if cfg := o.Services[serviceID]; cfg != nil {
		return cfg
	}
	cfg := &proc.Config{}
	o.Services[serviceID] = cfg
	return cfg
}

// ServiceConfig returns a copy of the per-service config if available.
func (o *BootOptions) ServiceConfig(serviceID proc.ServiceID) (proc.Config, bool) {
	if o == nil || serviceID == "" {
		return proc.Config{}, false
	}
	cfg := o.Service(serviceID)
	if cfg == nil {
		return proc.Config{}, false
	}
	return *cfg, true
}

// SortedServiceIDs returns all configured service IDs in deterministic order.
func (o *BootOptions) SortedServiceIDs() []proc.ServiceID {
	if o == nil || len(o.Services) == 0 {
		return nil
	}
	out := make([]proc.ServiceID, 0, len(o.Services))
	for id := range o.Services {
		out = append(out, id)
	}
	slices.SortStableFunc(out, func(a, b proc.ServiceID) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})
	return out
}

// SharedOptions returns the boot-time shared options.
func (o *BootOptions) SharedOptions() proc.SharedOptions {
	if o == nil {
		return proc.SharedOptions{}
	}
	return o.ShOpt
}

// BootVersion returns the boot-time version constraint.
func (o *BootOptions) BootVersion() string {
	if o == nil {
		return ""
	}
	return o.Version
}

// MonitorEnabled reports whether monitoring components are enabled.
func (o *BootOptions) MonitorEnabled() bool {
	return o != nil && o.Monitor
}

// GrafanaPortOverride returns the configured Grafana port override.
func (o *BootOptions) GrafanaPortOverride() int {
	if o == nil {
		return 0
	}
	return o.GrafanaPort
}

// ServiceConfigFor returns the current config snapshot for a service.
func (o *BootOptions) ServiceConfigFor(serviceID proc.ServiceID) proc.Config {
	if o == nil || serviceID == "" {
		return proc.Config{}
	}
	cfg := o.Service(serviceID)
	if cfg == nil {
		return proc.Config{}
	}
	return *cfg
}

func parseS3Endpoint(endpoint string) (host string, secure bool, hostname string, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if !strings.HasPrefix(endpoint, "https://") && !strings.HasPrefix(endpoint, "http://") {
		return "", false, "", fmt.Errorf("require S3 endpoint to start with http:// or https://")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", false, "", fmt.Errorf("invalid S3 endpoint: %w", err)
	}
	if u.Host == "" {
		return "", false, "", fmt.Errorf("require S3 endpoint to include a host")
	}
	if u.Path != "" && u.Path != "/" {
		return "", false, "", fmt.Errorf("require S3 endpoint to not include a path")
	}
	if u.RawQuery != "" {
		return "", false, "", fmt.Errorf("require S3 endpoint to not include query parameters")
	}
	if u.Fragment != "" {
		return "", false, "", fmt.Errorf("require S3 endpoint to not include fragment")
	}

	switch u.Scheme {
	case "http":
		secure = false
	case "https":
		secure = true
	default:
		return "", false, "", fmt.Errorf("require S3 endpoint to start with http:// or https://")
	}

	return u.Host, secure, u.Hostname(), nil
}

func normalizeBootErr(ctx context.Context, err error) error {
	if err == nil || ctx == nil {
		return err
	}

	if stdErrors.Is(err, context.Canceled) || errors.Cause(err) == context.Canceled {
		cause := context.Cause(ctx)
		if cause != nil && cause != context.Canceled {
			return cause
		}
	}
	return err
}

func (p *Playground) cancelBootWithCause(cause error) {
	if p == nil {
		return
	}
	cancel := p.bootCancel
	if cancel != nil {
		cancel(cause)
	}
}

func normalizeBootOptionPaths(options *BootOptions) error {
	if options == nil {
		return nil
	}

	for _, serviceID := range options.SortedServiceIDs() {
		cfg := options.Service(serviceID)
		if cfg == nil {
			continue
		}
		binPath, err := getAbsolutePath(cfg.BinPath)
		if err != nil {
			return errors.Annotatef(err, "cannot eval absolute directory: %s", cfg.BinPath)
		}
		cfg.BinPath = binPath

		path, err := getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return errors.Annotatef(err, "cannot eval absolute directory: %s", cfg.ConfigPath)
		}
		cfg.ConfigPath = path
	}

	return nil
}

// ValidateBootOptionsPure performs pure validation that must be shared by both
// dry-run and normal execution.
//
// It must not perform network calls or other external side effects.
func ValidateBootOptionsPure(options *BootOptions) error {
	if options == nil {
		return nil
	}

	if strings.TrimSpace(options.Host) == "" {
		return fmt.Errorf("host is empty")
	}

	cfgPD := options.Service(proc.ServicePD)
	cfgDMMaster := options.Service(proc.ServiceDMMaster)

	if err := validateServiceCountLimits(options); err != nil {
		return err
	}

	// All other components depend on PD, except DM. Ensure PD count > 0 for the
	// common modes.
	if options.ShOpt.PDMode != "ms" && cfgPD != nil && cfgPD.Num < 1 && cfgDMMaster != nil && cfgDMMaster.Num < 1 {
		return fmt.Errorf("all components count must be great than 0 (pd=%v)", cfgPD.Num)
	}

	if options.ShOpt.PDMode == "ms" && utils.Version(options.Version).IsValid() && !tidbver.PDSupportMicroservices(options.Version) {
		return fmt.Errorf("PD cluster doesn't support microservices mode in version %s", options.Version)
	}

	switch options.ShOpt.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
		if utils.Version(options.Version).IsValid() && !tidbver.TiFlashPlaygroundNewStartMode(options.Version) {
			// For simplicity, currently we only implemented disagg mode when TiFlash can run without config.
			return fmt.Errorf("tiup playground-ng only supports CSE/Disagg mode for TiDB cluster >= v7.1.0 (or nightly)")
		}
	}

	switch options.ShOpt.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
		bucket := strings.TrimSpace(options.ShOpt.CSE.Bucket)
		if bucket == "" {
			return fmt.Errorf("require S3 bucket to be non-empty")
		}
		_, _, hostname, err := parseS3Endpoint(options.ShOpt.CSE.S3Endpoint)
		if err != nil {
			return err
		}

		// Currently we always assign region=local. Other regions are not supported.
		if strings.Contains(hostname, "amazonaws.com") {
			return fmt.Errorf("tiup playground-ng only supports local S3 (like minio); S3 on AWS regions is not supported")
		}
	}

	return nil
}

func validateServiceCountLimits(options *BootOptions) error {
	if options == nil {
		return nil
	}

	for _, spec := range pgservice.AllSpecs() {
		maxNum := spec.Catalog.MaxNum
		if spec.ServiceID == "" || maxNum <= 0 {
			continue
		}

		cfg := options.Service(spec.ServiceID)
		if cfg == nil || cfg.Num <= maxNum {
			continue
		}

		name := proc.ServiceDisplayName(spec.ServiceID)
		if name == "" {
			name = spec.ServiceID.String()
		}
		return fmt.Errorf("%s only supports at most %d instance(s)", name, maxNum)
	}

	return nil
}

func planProcs(options *BootOptions) ([]proc.ServiceID, map[proc.ServiceID]proc.Config, error) {
	if options == nil {
		return nil, nil, nil
	}

	cfgByService := make(map[proc.ServiceID]proc.Config)
	var serviceIDs []proc.ServiceID

	for _, spec := range pgservice.AllSpecs() {
		def := spec.Catalog
		if def.IsEnabled == nil || !def.IsEnabled(options) {
			continue
		}

		cfg := proc.Config{}
		if def.PlanConfig != nil {
			cfg = def.PlanConfig(options)
		} else {
			cfg, _ = options.ServiceConfig(spec.ServiceID)
		}

		cfgByService[spec.ServiceID] = cfg
		if cfg.Num > 0 {
			serviceIDs = append(serviceIDs, spec.ServiceID)
		}
	}

	type serviceStartKey struct {
		hasUserBin bool
		critical   bool
		id         string
	}
	keys := make(map[proc.ServiceID]serviceStartKey, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		cfg := cfgByService[serviceID]
		key := serviceStartKey{hasUserBin: cfg.BinPath != "", id: serviceID.String()}
		if cfg.Num > 0 {
			if spec, ok := pgservice.SpecFor(serviceID); ok && spec.Catalog.IsCritical != nil && spec.Catalog.IsCritical(options) {
				key.critical = true
			}
		}
		keys[serviceID] = key
	}

	ordered, err := topoSortServiceIDsWithCompare(serviceIDs, func(a, b proc.ServiceID) int {
		ka := keys[a]
		kb := keys[b]
		switch {
		case ka.hasUserBin != kb.hasUserBin:
			if ka.hasUserBin {
				return -1
			}
			return 1
		case ka.critical != kb.critical:
			if ka.critical {
				return -1
			}
			return 1
		default:
			if ka.id == "" {
				ka.id = a.String()
			}
			if kb.id == "" {
				kb.id = b.String()
			}
			return strings.Compare(ka.id, kb.id)
		}
	})
	if err != nil {
		return nil, nil, err
	}
	return ordered, cfgByService, nil
}

type envComponentSource struct {
	env *environment.Environment
}

func newEnvComponentSource(env *environment.Environment) *envComponentSource {
	return &envComponentSource{env: env}
}

func (s *envComponentSource) ResolveVersion(component, constraint string) (string, error) {
	if s == nil || s.env == nil {
		return "", errors.New("environment not initialized")
	}
	v, err := s.env.V1Repository().ResolveComponentVersion(component, constraint)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

func requiredBinaryPathForService(serviceID proc.ServiceID, baseBinPath string) string {
	baseBinPath = strings.TrimSpace(baseBinPath)
	if baseBinPath == "" {
		return ""
	}

	switch serviceID {
	case proc.ServiceTiKVWorker:
		return proc.ResolveTiKVWorkerBinPath(baseBinPath)
	case proc.ServiceNGMonitoring:
		if filepath.Base(baseBinPath) == "ng-monitoring-server" {
			return baseBinPath
		}
		path, _ := proc.ResolveSiblingBinary(baseBinPath, "ng-monitoring-server")
		return path
	default:
		return baseBinPath
	}
}

func planInstallByResolvedBinaryPath(serviceID proc.ServiceID, component, resolved, baseBinPath string, binPathErr error, forcePull bool) *DownloadPlan {
	// PlanInstall treats any binary path error as "not installed": it should
	// produce a download plan rather than failing planning.
	if binPathErr == nil && !forcePull {
		checkPath := requiredBinaryPathForService(serviceID, baseBinPath)
		if binaryExists(checkPath) {
			return nil
		}
	}

	reason := "not_installed"
	if forcePull {
		reason = "force_pull"
	} else if binPathErr == nil {
		reason = "missing_binary"
	}

	debugBinPath := strings.TrimSpace(baseBinPath)
	if binPathErr == nil {
		debugBinPath = strings.TrimSpace(requiredBinaryPathForService(serviceID, baseBinPath))
	}

	return &DownloadPlan{
		ComponentID:     component,
		ResolvedVersion: resolved,
		DebugReason:     reason,
		DebugBinPath:    debugBinPath,
	}
}

func (s *envComponentSource) PlanInstall(serviceID proc.ServiceID, component, resolved string, forcePull bool) (*DownloadPlan, error) {
	if s == nil || s.env == nil {
		return nil, errors.New("environment not initialized")
	}
	if component == "" {
		return nil, errors.New("component is empty")
	}
	if resolved == "" {
		return nil, errors.Errorf("component %s resolved version is empty", component)
	}

	v := utils.Version(resolved)
	binPath, err := s.env.BinaryPath(component, v)
	return planInstallByResolvedBinaryPath(serviceID, component, resolved, binPath, err, forcePull), nil
}

func (s *envComponentSource) EnsureInstalled(component, resolved string) error {
	if s == nil || s.env == nil {
		return errors.New("environment not initialized")
	}
	if component == "" {
		return errors.New("component is empty")
	}
	if resolved == "" {
		return errors.Errorf("component %s resolved version is empty", component)
	}
	spec := repository.ComponentSpec{ID: component, Version: resolved, Force: true}
	return s.env.V1Repository().UpdateComponents([]repository.ComponentSpec{spec})
}

func (s *envComponentSource) BinaryPath(component, resolved string) (string, error) {
	if s == nil || s.env == nil {
		return "", errors.New("environment not initialized")
	}
	if component == "" {
		return "", errors.New("component is empty")
	}
	if resolved == "" {
		return "", errors.Errorf("component %s resolved version is empty", component)
	}
	return s.env.BinaryPath(component, utils.Version(resolved))
}

func (p *Playground) bootCluster(ctx context.Context, options *BootOptions) (err error) {
	defer func() { err = normalizeBootErr(ctx, err) }()

	if err := normalizeBootOptionPaths(options); err != nil {
		return err
	}

	p.bootOptions = options
	// Start the controller early so instance lifecycle events (started/exited)
	// can be handled via the actor loop during boot.
	p.startController()
	p.setControllerBooting(ctx, true)
	defer p.setControllerBooting(context.Background(), false)

	if err := ValidateBootOptionsPure(options); err != nil {
		return err
	}

	orderedServiceIDs, baseConfigs, err := planProcs(options)
	if err != nil {
		return err
	}

	src := newEnvComponentSource(environment.GlobalEnv())
	plan, err := buildBootPlanWithProcs(options, bootPlannerConfig{
		dataDir:            p.dataDir,
		portConflictPolicy: PortConflictAllocFree,
		componentSource:    src,
	}, orderedServiceIDs, baseConfigs)
	if err != nil {
		return err
	}
	p.bootBaseConfigs = make(map[proc.ServiceID]proc.Config, len(baseConfigs))
	for serviceID, cfg := range baseConfigs {
		if serviceID == "" {
			continue
		}
		p.bootBaseConfigs[serviceID] = cfg
	}

	required := make(map[proc.ServiceID]int, len(plan.RequiredServices))
	for serviceID, min := range plan.RequiredServices {
		id := proc.ServiceID(serviceID)
		if id == "" || min <= 0 {
			continue
		}
		required[id] = min
	}
	p.setControllerRequiredServices(ctx, required)

	if len(plan.Downloads) > 0 {
		p.progressMu.Lock()
		downloadProgress := p.downloadProgress
		p.progressMu.Unlock()
		if downloadProgress != nil {
			downloadProgress.SetExpectedDownloads(plan.Downloads)
		}
	}

	executor := newBootExecutor(p, src)
	if err := executor.Download(plan); err != nil {
		return err
	}
	if len(plan.Downloads) > 0 {
		p.progressMu.Lock()
		downloadGroup := p.downloadGroup
		p.progressMu.Unlock()
		if downloadGroup != nil {
			downloadGroup.Close()
		}
	}
	if err := executor.PreRun(ctx, plan); err != nil {
		return err
	}
	if err := executor.AddProcs(ctx, plan); err != nil {
		return err
	}

	planned := p.procsSnapshot()
	p.initBootStartingTasks()

	starter := newBootStarter(ctx, p, planned, required)
	ready, err := starter.startPlanned(plannedServicesFromBootPlan(plan))
	if err != nil {
		return err
	}

	// Ensure critical services become ready before concluding boot. This is
	// especially important for modes like TiKV-slim where TiDB is not started and
	// thus won't implicitly wait for TiKV readiness via StartAfter.
	if err := starter.waitRequiredReady(); err != nil {
		return err
	}

	tidbSucc := starter.waitReadyAddrs(ready[proc.ServiceTiDB])
	tiproxySucc := starter.waitReadyAddrs(ready[proc.ServiceTiProxy])

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Ensure "Start instances" tasks reach their terminal states before we close
	// the progress group and print user-facing hints, otherwise the output blocks
	// may interleave in daemon/starter mode (extra blank lines, wrong ordering).
	p.waitBootStartingTasksSettled(time.Second)

	// Conclude "Start instances" before printing user-facing hints, so the
	// final group output stays in the history area and won't be redrawn.
	p.closeStartingGroup()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if p.ui != nil {
		p.ui.PrintLines([]string{""})
	} else {
		fmt.Fprintln(p.terminalWriter())
	}
	_ = p.printClusterInfoCallout(tidbSucc, tiproxySucc)

	tidbDSN := pgservice.ProcsOf[*proc.TiDBInstance](p, proc.ServiceTiDB)
	tiproxyDSN := pgservice.ProcsOf[*proc.TiProxyInstance](p, proc.ServiceTiProxy)
	dumpDSN(filepath.Join(p.dataDir, "dsn"), tidbDSN, tiproxyDSN)

	logIfErr(p.renderSDFile())

	if ps := pgservice.ProcsOf[*proc.PrometheusInstance](p, proc.ServicePrometheus); len(ps) > 0 && ps[0] != nil {
		p.updateMonitorTopology(spec.ComponentPrometheus, MonitorInfo{IP: ps[0].Host, Port: ps[0].Port, BinaryPath: ps[0].BinPath})
	}
	if gs := pgservice.ProcsOf[*proc.GrafanaInstance](p, proc.ServiceGrafana); len(gs) > 0 && gs[0] != nil {
		p.updateMonitorTopology(spec.ComponentGrafana, MonitorInfo{IP: gs[0].Host, Port: gs[0].Port, BinaryPath: gs[0].BinPath})
	}

	// Mark boot as completed before starting the HTTP command server, so
	// subsequent scale-out operations can follow the "join" path.
	p.setControllerBooted(context.Background(), true)

	// Start the HTTP command server last, after all post-start
	// artifacts (sd file, dsn, topology hints) are ready.
	if p.processGroup != nil {
		_ = p.processGroup.Add("command server", func() error {
			// fmt.Printf("serve at :%d\n", p.port)
			err := p.listenAndServeHTTP()
			if err != nil {
				fmt.Fprintf(p.terminalWriter(), "listenAndServeHTTP quit: %s\n", err)
			}
			return err
		})
	} else {
		go func() {
			// fmt.Printf("serve at :%d\n", p.port)
			err := p.listenAndServeHTTP()
			if err != nil {
				fmt.Fprintf(p.terminalWriter(), "listenAndServeHTTP quit: %s\n", err)
			}
		}()
	}

	return nil
}

func topoSortServiceIDs(serviceIDs []proc.ServiceID) ([]proc.ServiceID, error) {
	return topoSortServiceIDsWithCompare(serviceIDs, nil)
}

type serviceIDCompareFunc func(a, b proc.ServiceID) int

func topoSortServiceIDsWithCompare(serviceIDs []proc.ServiceID, cmp serviceIDCompareFunc) ([]proc.ServiceID, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	if cmp == nil {
		cmp = func(a, b proc.ServiceID) int {
			return strings.Compare(a.String(), b.String())
		}
	}

	inSet := make(map[proc.ServiceID]struct{}, len(serviceIDs))
	for _, id := range serviceIDs {
		inSet[id] = struct{}{}
	}

	indegree := make(map[proc.ServiceID]int, len(serviceIDs))
	deps := make(map[proc.ServiceID][]proc.ServiceID, len(serviceIDs))
	for _, id := range serviceIDs {
		indegree[id] = 0
	}

	for _, id := range serviceIDs {
		spec, ok := pgservice.SpecFor(id)
		if !ok {
			continue
		}
		for _, dep := range spec.StartAfter {
			if _, ok := inSet[dep]; !ok {
				continue
			}
			deps[dep] = append(deps[dep], id)
			indegree[id]++
		}
	}

	var ready []proc.ServiceID
	for _, id := range serviceIDs {
		if indegree[id] == 0 {
			ready = append(ready, id)
		}
	}
	slices.SortStableFunc(ready, func(a, b proc.ServiceID) int {
		return cmp(a, b)
	})

	out := make([]proc.ServiceID, 0, len(serviceIDs))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		out = append(out, id)

		for _, next := range deps[id] {
			indegree[next]--
			if indegree[next] == 0 {
				ready = append(ready, next)
			}
		}
		if len(ready) > 1 {
			slices.SortStableFunc(ready, func(a, b proc.ServiceID) int {
				return cmp(a, b)
			})
		}
	}

	if len(out) != len(serviceIDs) {
		var cycle []proc.ServiceID
		for _, id := range serviceIDs {
			if indegree[id] > 0 {
				cycle = append(cycle, id)
			}
		}
		slices.SortStableFunc(cycle, func(a, b proc.ServiceID) int {
			return cmp(a, b)
		})
		var names []string
		for _, id := range cycle {
			names = append(names, id.String())
		}
		return nil, fmt.Errorf("service dependency cycle detected: %s", strings.Join(names, ", "))
	}

	return out, nil
}
