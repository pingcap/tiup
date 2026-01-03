package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/utils"
)

type readyFuture struct {
	ch   <-chan error
	once sync.Once
	err  error
}

func (f *readyFuture) Wait() error {
	if f == nil || f.ch == nil {
		return nil
	}
	f.once.Do(func() {
		f.err = <-f.ch
	})
	return f.err
}

type readyAddr struct {
	addr  string
	ready *readyFuture
}

type bootStarter struct {
	pg       *Playground
	ctx      context.Context
	preload  *binaryPreloader
	anyPump  bool
	readyMap map[proc.ServiceID][]*readyFuture
}

func newBootStarter(pg *Playground, ctx context.Context, preload *binaryPreloader) *bootStarter {
	if ctx == nil {
		ctx = context.Background()
	}
	return &bootStarter{
		pg:       pg,
		ctx:      ctx,
		preload:  preload,
		readyMap: make(map[proc.ServiceID][]*readyFuture),
	}
}

func (b *bootStarter) waitStartAfter(serviceID proc.ServiceID) error {
	if b == nil || b.pg == nil {
		return nil
	}
	spec, ok := pgservice.SpecFor(serviceID)
	if !ok || len(spec.StartAfter) == 0 {
		return nil
	}

	for _, dep := range spec.StartAfter {
		if len(b.pg.procs[dep]) == 0 {
			continue
		}
		readyList := b.readyMap[dep]
		if len(readyList) == 0 {
			return fmt.Errorf("%s requires %s started", serviceID, dep)
		}
		for _, f := range readyList {
			if err := f.Wait(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *bootStarter) startProc(serviceID proc.ServiceID, inst proc.Process) (*readyFuture, error) {
	if b == nil || b.pg == nil || inst == nil {
		return nil, nil
	}

	info := inst.Info()
	if info == nil {
		return nil, fmt.Errorf("instance %T has nil info", inst)
	}

	critical := b.pg.isRequiredService(serviceID)

	if bin := info.UserBinPath; bin != "" {
		info.BinPath = bin
		info.Version = utils.Version("")
	} else {
		constraint, binPath, version, err := b.preload.resolve(info.RepoComponentID.String())
		if err != nil {
			if critical {
				return nil, err
			}
			b.pg.markStartingTaskError(inst, constraint, err)
			return nil, nil
		}
		info.BinPath = binPath
		info.Version = version
	}

	readyCh, err := b.pg.startProc(b.ctx, inst)
	if err != nil {
		if critical {
			return nil, err
		}
		// Non-critical components are best-effort: keep starting others.
		return nil, nil
	}

	f := &readyFuture{ch: readyCh}
	b.readyMap[serviceID] = append(b.readyMap[serviceID], f)

	// If no pump becomes ready, TiDB will quit right away.
	if serviceID == proc.ServicePump && !b.anyPump {
		if err := f.Wait(); err != nil {
			return nil, err
		}
		b.anyPump = true
	}

	return f, nil
}

func (b *bootStarter) startPlanned(plans []plannedProc) (tidbReady, tiproxyReady []readyAddr, err error) {
	if b == nil || b.pg == nil {
		return nil, nil, nil
	}

	for _, plan := range plans {
		serviceID := plan.serviceID
		switch serviceID {
		case proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute:
			continue
		}

		if err := b.waitStartAfter(serviceID); err != nil {
			if b.pg.isRequiredService(serviceID) {
				return nil, nil, err
			}
			for _, inst := range b.pg.procs[serviceID] {
				b.pg.markStartingTaskError(inst, "", err)
			}
			continue
		}

		for _, inst := range b.pg.procs[serviceID] {
			f, err := b.startProc(serviceID, inst)
			if err != nil {
				return nil, nil, err
			}
			if f == nil {
				continue
			}

			switch serviceID {
			case proc.ServiceTiDB:
				if tdb, ok := inst.(*proc.TiDBInstance); ok {
					tidbReady = append(tidbReady, readyAddr{addr: tdb.Addr(), ready: f})
				}
			case proc.ServiceTiProxy:
				if proxy, ok := inst.(*proc.TiProxyInstance); ok {
					tiproxyReady = append(tiproxyReady, readyAddr{addr: proxy.Addr(), ready: f})
				}
			}
		}
	}

	return tidbReady, tiproxyReady, nil
}

func (b *bootStarter) waitReadyAddrs(rs []readyAddr) (succ []string) {
	for _, r := range rs {
		if r.ready != nil {
			if err := r.ready.Wait(); err == nil {
				succ = append(succ, r.addr)
			}
			continue
		}
		// Best-effort fallback: treat "no ready check" as ready.
		succ = append(succ, r.addr)
	}
	return succ
}
