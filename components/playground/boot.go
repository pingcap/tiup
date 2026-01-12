package main

import (
	"context"
	stdErrors "errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

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

	if options.ShOpt.PDMode == "ms" && !tidbver.PDSupportMicroservices(options.Version) {
		return fmt.Errorf("PD cluster doesn't support microservices mode in version %s", options.Version)
	}

	switch options.ShOpt.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
		if utils.Version(options.Version).IsValid() && !tidbver.TiFlashPlaygroundNewStartMode(options.Version) {
			// For simplicity, currently we only implemented disagg mode when TiFlash can run without config.
			return fmt.Errorf("TiUP playground only supports CSE/Disagg mode for TiDB cluster >= v7.1.0 (or nightly)")
		}
	}

	switch options.ShOpt.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
		if !strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "https://") && !strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "http://") {
			return fmt.Errorf("require S3 endpoint to start with http:// or https://")
		}

		rawEndpoint := strings.TrimPrefix(options.ShOpt.CSE.S3Endpoint, "https://")
		rawEndpoint = strings.TrimPrefix(rawEndpoint, "http://")

		// Currently we always assign region=local. Other regions are not supported.
		if strings.Contains(rawEndpoint, "amazonaws.com") {
			return fmt.Errorf("tiup playground only supports local S3 (like minio); S3 on AWS regions is not supported")
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

	src := newEnvComponentSource(environment.GlobalEnv())
	plan, err := BuildBootPlan(options, bootPlannerConfig{
		dataDir:            p.dataDir,
		portConflictPolicy: PortConflictAllocFree,
		componentSource:    src,
	})
	if err != nil {
		return err
	}
	_, baseConfigs, err := planProcs(options)
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

	// Conclude "Starting instances" before printing user-facing hints, so the
	// final group output stays in the history area and won't be redrawn.
	p.closeStartingGroup()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if p.ui != nil {
		p.ui.BlankLine()
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
	go func() {
		// fmt.Printf("serve at :%d\n", p.port)
		err := p.listenAndServeHTTP()
		if err != nil {
			fmt.Fprintf(p.terminalWriter(), "listenAndServeHTTP quit: %s\n", err)
		}
	}()

	return nil
}
