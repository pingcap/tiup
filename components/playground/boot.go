package main

import (
	"context"
	stdErrors "errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/cluster/spec"
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

func (p *Playground) normalizeBootOptionPaths(options *BootOptions) error {
	if options == nil {
		return nil
	}

	for _, serviceID := range options.SortedServiceIDs() {
		cfg := options.Service(serviceID)
		if cfg == nil {
			continue
		}
		path, err := getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return errors.Annotatef(err, "cannot eval absolute directory: %s", cfg.ConfigPath)
		}
		cfg.ConfigPath = path
	}

	return nil
}

func (p *Playground) validateBootOptions(ctx context.Context, options *BootOptions) error {
	if p == nil || options == nil {
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

	switch options.ShOpt.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
		if !strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "https://") && !strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "http://") {
			return fmt.Errorf("require S3 endpoint to start with http:// or https://")
		}

		isSecure := strings.HasPrefix(options.ShOpt.CSE.S3Endpoint, "https://")
		rawEndpoint := strings.TrimPrefix(options.ShOpt.CSE.S3Endpoint, "https://")
		rawEndpoint = strings.TrimPrefix(rawEndpoint, "http://")

		// Currently we always assign region=local. Other regions are not supported.
		if strings.Contains(rawEndpoint, "amazonaws.com") {
			return fmt.Errorf("tiup playground only supports local S3 (like minio); S3 on AWS regions is not supported")
		}

		// Preflight check whether specified object storage is available.
		s3Client, err := minio.New(rawEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(options.ShOpt.CSE.AccessKey, options.ShOpt.CSE.SecretKey, ""),
			Secure: isSecure,
		})
		if err != nil {
			return errors.Annotate(err, "can not connect to S3 endpoint")
		}

		ctxCheck, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		bucketExists, err := s3Client.BucketExists(ctxCheck, options.ShOpt.CSE.Bucket)
		if err != nil {
			return errors.Annotate(err, "can not connect to S3 endpoint")
		}
		if !bucketExists {
			// Try to create bucket.
			if err := s3Client.MakeBucket(ctxCheck, options.ShOpt.CSE.Bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("cannot create s3 bucket: Bucket %s doesn't exist and fail to create automatically (your bucket name may be invalid?)", options.ShOpt.CSE.Bucket)
			}
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

type plannedProc struct {
	serviceID proc.ServiceID
	cfg       proc.Config
}

type bootPlan struct {
	Plans []plannedProc

	// BaseConfigs holds the per-service config snapshot decided during planning.
	// It is used for boot-time defaults and scale-out request sanitization.
	BaseConfigs map[proc.ServiceID]proc.Config

	// RequiredServices is the minimum running instance count for "critical"
	// services. Controller uses it to trigger auto shutdown if critical services
	// exit unexpectedly.
	RequiredServices map[proc.ServiceID]int
}

func buildBootPlan(options *BootOptions) (bootPlan, error) {
	plans, baseConfigs, err := planProcs(options)
	if err != nil {
		return bootPlan{}, err
	}

	if baseConfigs == nil {
		baseConfigs = make(map[proc.ServiceID]proc.Config)
	}

	required := make(map[proc.ServiceID]int)
	for serviceID, cfg := range baseConfigs {
		if serviceID == "" || cfg.Num <= 0 || options == nil {
			continue
		}
		spec, ok := pgservice.SpecFor(serviceID)
		if !ok {
			continue
		}
		if spec.Catalog.IsCritical != nil && spec.Catalog.IsCritical(options) {
			required[serviceID] = 1
		}
	}

	return bootPlan{
		Plans:            plans,
		BaseConfigs:      baseConfigs,
		RequiredServices: required,
	}, nil
}

func planProcs(options *BootOptions) ([]plannedProc, map[proc.ServiceID]proc.Config, error) {
	if options == nil {
		return nil, nil, nil
	}

	if options.ShOpt.PDMode == "ms" && !tidbver.PDSupportMicroservices(options.Version) {
		return nil, nil, fmt.Errorf("PD cluster doesn't support microservices mode in version %s", options.Version)
	}

	if options.ShOpt.Mode == proc.ModeCSE || options.ShOpt.Mode == proc.ModeNextGen || options.ShOpt.Mode == proc.ModeDisAgg {
		if utils.Version(options.Version).IsValid() && !tidbver.TiFlashPlaygroundNewStartMode(options.Version) {
			// For simplicity, currently we only implemented disagg mode when TiFlash can run without config.
			return nil, nil, fmt.Errorf("TiUP playground only supports CSE/Disagg mode for TiDB cluster >= v7.1.0 (or nightly)")
		}
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

	plans := make([]plannedProc, 0, len(ordered))
	for _, serviceID := range ordered {
		plans = append(plans, plannedProc{serviceID: serviceID, cfg: cfgByService[serviceID]})
	}
	return plans, cfgByService, nil
}

func (p *Playground) addPlannedProcs(ctx context.Context, plans []plannedProc) error {
	if p == nil {
		return nil
	}
	for _, plan := range plans {
		for i := 0; i < plan.cfg.Num; i++ {
			_, err := p.requestAddProc(ctx, plan.serviceID, plan.cfg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Playground) bootCluster(ctx context.Context, options *BootOptions) (err error) {
	defer func() { err = normalizeBootErr(ctx, err) }()

	if err := p.normalizeBootOptionPaths(options); err != nil {
		return err
	}

	p.bootOptions = options
	// Start the controller early so instance lifecycle events (started/exited)
	// can be handled via the actor loop during boot.
	p.startController()
	p.setControllerBooting(ctx, true)
	defer p.setControllerBooting(context.Background(), false)

	if err := p.validateBootOptions(ctx, options); err != nil {
		return err
	}

	plan, err := buildBootPlan(options)
	if err != nil {
		return err
	}

	p.bootBaseConfigs = plan.BaseConfigs
	required := plan.RequiredServices
	p.setControllerRequiredServices(ctx, required)

	if err := p.addPlannedProcs(ctx, plan.Plans); err != nil {
		return err
	}

	planned := p.procsSnapshot()
	startingTasks := p.initBootStartingTasks()

	p.progressMu.Lock()
	downloadGroup := p.downloadGroup
	p.progressMu.Unlock()

	// Kick off component downloads early so "Downloading components" and
	// "Starting instances" can overlap.
	//
	// Playground knows the full component set upfront. Prefetching binaries here
	// avoids the previous "start some instances -> download -> start more"
	// behavior.
	preloader := newBinaryPreloader(ctx, p, p.bootOptions.Version, p.bootOptions.ShOpt.ForcePull)
	preloader.collect(startingTasks)
	preloader.start()

	// Close the download group once all prefetches finish. This lets the UI
	// collapse successful downloads while instance startup continues.
	if downloadGroup != nil && len(preloader.items) > 0 {
		go func() {
			<-preloader.allDone()
			downloadGroup.Close()
		}()
	}

	starter := newBootStarter(ctx, p, preloader, planned, required)
	ready, err := starter.startPlanned(plan.Plans)
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
