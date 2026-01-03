package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/cluster/spec"
)

func (p *Playground) bootCluster(ctx context.Context, options *BootOptions) (err error) {
	defer func() { err = normalizeBootErr(ctx, err) }()

	if err := p.normalizeBootOptionPaths(options); err != nil {
		return err
	}

	p.bootOptions = options
	p.setRequiredServices(options)
	// Start the controller early so instance lifecycle events (started/exited)
	// can be handled via the actor loop during boot.
	p.startController(ctx)
	p.setControllerBooting(ctx, true)
	defer p.setControllerBooting(context.Background(), false)

	if err := p.validateBootOptions(ctx, options); err != nil {
		return err
	}

	plans, err := planProcs(options)
	if err != nil {
		return err
	}
	p.bootBaseConfigs = make(map[proc.ServiceID]proc.Config, len(plans))
	for _, plan := range plans {
		p.bootBaseConfigs[plan.serviceID] = plan.cfg
	}
	if err := p.addPlannedProcs(plans); err != nil {
		return err
	}

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

	starter := newBootStarter(p, ctx, preloader)
	tidbReady, tiproxyReady, err := starter.startPlanned(plans)
	if err != nil {
		return err
	}

	tidbSucc := starter.waitReadyAddrs(tidbReady)
	tiproxySucc := starter.waitReadyAddrs(tiproxyReady)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// TiDB is a critical component for all modes that include it. If none becomes
	// ready, abort boot and stop the cluster instead of continuing to start
	// optional components.
	if options.ShOpt.Mode != proc.ModeTiKVSlim && options.Service(proc.ServiceTiDB).Num > 0 && len(tidbSucc) == 0 {
		return fmt.Errorf("no TiDB instance became ready")
	}

	if len(tidbSucc) > 0 {
		if err := p.startTiFlashAfterTiDB(ctx, preloader); err != nil {
			return err
		}
	} else if ctx.Err() == nil {
		p.skipTiFlashStartingTasks("no TiDB ready")
	}

	// Conclude "Starting instances" before printing user-facing hints, so the
	// final group output stays in the history area and won't be redrawn.
	p.closeStartingGroup()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	fmt.Fprintln(p.termWriter())
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
	p.booted = true

	// Start the HTTP command server last, after all post-start
	// artifacts (sd file, dsn, topology hints) are ready.
	go func() {
		// fmt.Printf("serve at :%d\n", p.port)
		err := p.listenAndServeHTTP()
		if err != nil {
			fmt.Fprintf(p.termWriter(), "listenAndServeHTTP quit: %s\n", err)
		}
	}()

	return nil
}
