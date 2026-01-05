package main

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

type readyFuture struct {
	done chan struct{}
	err  error
}

func newReadyFuture(ch <-chan error) *readyFuture {
	f := &readyFuture{done: make(chan struct{})}
	go func() {
		f.err = <-ch
		close(f.done)
	}()
	return f
}

func (f *readyFuture) Done() <-chan struct{} {
	if f == nil || f.done == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return f.done
}

func (f *readyFuture) Err() error {
	if f == nil {
		return nil
	}
	return f.err
}

func (f *readyFuture) Wait() error {
	if f == nil {
		return nil
	}
	<-f.Done()
	return f.Err()
}

type readyAddr struct {
	addr  string
	ready *readyFuture
}

type readySet map[proc.ServiceID][]readyAddr

type bootStarter struct {
	pg       *Playground
	ctx      context.Context
	preload  *binaryPreloader
	planned  map[proc.ServiceID][]proc.Process
	required map[proc.ServiceID]int
	readyMap map[proc.ServiceID][]*readyFuture
	readySet readySet
}

func newBootStarter(pg *Playground, ctx context.Context, preload *binaryPreloader, planned map[proc.ServiceID][]proc.Process, required map[proc.ServiceID]int) *bootStarter {
	if ctx == nil {
		ctx = context.Background()
	}
	return &bootStarter{
		pg:       pg,
		ctx:      ctx,
		preload:  preload,
		planned:  planned,
		required: required,
		readyMap: make(map[proc.ServiceID][]*readyFuture),
		readySet: make(readySet),
	}
}

func (b *bootStarter) isRequiredService(serviceID proc.ServiceID) bool {
	if b == nil || serviceID == "" {
		return false
	}
	return b.required[serviceID] > 0
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
		if len(b.planned[dep]) == 0 {
			continue
		}
		readyList := b.readyMap[dep]
		if len(readyList) == 0 {
			return fmt.Errorf("%s requires %s started", serviceID, dep)
		}
		if err := waitAnyReady(readyList); err != nil {
			return err
		}
	}
	return nil
}

func waitAnyReady(fs []*readyFuture) error {
	if len(fs) == 0 {
		return fmt.Errorf("no instance became ready")
	}

	var lastErr error

	pending := make([]*readyFuture, 0, len(fs))
	cases := make([]reflect.SelectCase, 0, len(fs))

	for _, f := range fs {
		if f == nil {
			return nil
		}
		done := f.Done()
		select {
		case <-done:
			if f.Err() == nil {
				return nil
			}
			lastErr = f.Err()
		default:
			pending = append(pending, f)
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(done)})
		}
	}

	for len(cases) > 0 {
		i, _, _ := reflect.Select(cases)
		f := pending[i]
		if f.Err() == nil {
			return nil
		}
		lastErr = f.Err()

		pending[i] = pending[len(pending)-1]
		cases[i] = cases[len(cases)-1]
		pending = pending[:len(pending)-1]
		cases = cases[:len(cases)-1]
	}

	if lastErr == nil {
		return fmt.Errorf("no instance became ready")
	}
	return lastErr
}

func (b *bootStarter) startProc(serviceID proc.ServiceID, inst proc.Process) (*readyFuture, error) {
	if b == nil || b.pg == nil || inst == nil {
		return nil, nil
	}

	critical := b.isRequiredService(serviceID)

	readyCh, err := b.pg.requestStartProc(b.ctx, inst, b.preload)
	if err != nil {
		if critical {
			return nil, err
		}
		// Non-critical components are best-effort: keep starting others.
		return nil, nil
	}

	f := newReadyFuture(readyCh)
	b.readyMap[serviceID] = append(b.readyMap[serviceID], f)
	if a, ok := inst.(interface{ Addr() string }); ok {
		if addr := a.Addr(); addr != "" {
			b.readySet[serviceID] = append(b.readySet[serviceID], readyAddr{addr: addr, ready: f})
		}
	}

	return f, nil
}

func (b *bootStarter) waitRequiredReady() error {
	if b == nil {
		return nil
	}

	var required []proc.ServiceID
	for serviceID, min := range b.required {
		if serviceID == "" || min <= 0 {
			continue
		}
		required = append(required, serviceID)
	}
	slices.SortStableFunc(required, func(a, b proc.ServiceID) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})

	for _, serviceID := range required {
		readyList := b.readyMap[serviceID]
		if len(readyList) == 0 {
			return fmt.Errorf("%s has no started instance", serviceID)
		}
		if err := waitAnyReady(readyList); err != nil {
			return fmt.Errorf("%s: %w", serviceID, err)
		}
	}
	return nil
}

func (b *bootStarter) startPlanned(plans []plannedProc) (readySet, error) {
	if b == nil || b.pg == nil {
		return nil, nil
	}

	for _, plan := range plans {
		serviceID := plan.serviceID

		if err := b.waitStartAfter(serviceID); err != nil {
			if b.isRequiredService(serviceID) {
				return nil, err
			}
			for _, inst := range b.planned[serviceID] {
				b.pg.markStartingTaskError(inst, "", err)
			}
			continue
		}

		for _, inst := range b.planned[serviceID] {
			f, err := b.startProc(serviceID, inst)
			if err != nil {
				return nil, err
			}
			if f == nil {
				continue
			}
		}
	}

	return b.readySet, nil
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
