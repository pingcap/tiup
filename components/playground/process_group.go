package main

import (
	"fmt"
	"sync"
)

var errProcessGroupClosed = fmt.Errorf("process group closed")

// ProcessGroup manages a dynamic set of long-running goroutines (process waiters,
// servers, watchers) that may be added during runtime (e.g. scale-out).
//
// Unlike errgroup.Group, Wait() is safe to call early because it blocks until
// Close() is called. This avoids the classic "wg.Add concurrent with wg.Wait"
// bug while still allowing dynamic additions during the running phase.
type ProcessGroup struct {
	mu       sync.Mutex
	closed   bool
	wg       sync.WaitGroup
	firstErr error

	closedCh chan struct{}
}

func NewProcessGroup() *ProcessGroup {
	return &ProcessGroup{
		closedCh: make(chan struct{}),
	}
}

func (g *ProcessGroup) Add(name string, wait func() error) error {
	if g == nil {
		return errProcessGroupClosed
	}
	if wait == nil {
		return fmt.Errorf("wait func is nil")
	}

	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return errProcessGroupClosed
	}
	g.wg.Add(1)
	g.mu.Unlock()

	go func() {
		defer g.wg.Done()
		if err := wait(); err != nil {
			g.mu.Lock()
			if g.firstErr == nil {
				if name != "" {
					g.firstErr = fmt.Errorf("%s: %w", name, err)
				} else {
					g.firstErr = err
				}
			}
			g.mu.Unlock()
		}
	}()
	return nil
}

func (g *ProcessGroup) Close() {
	if g == nil {
		return
	}
	g.mu.Lock()
	already := g.closed
	g.closed = true
	ch := g.closedCh
	g.mu.Unlock()
	if already {
		return
	}
	if ch != nil {
		close(ch)
	}
}

func (g *ProcessGroup) Wait() error {
	if g == nil {
		return nil
	}

	// Wait until the group is closed. This keeps Wait() safe to call early (it is
	// used to keep the playground process alive) while still allowing dynamic
	// additions during runtime (scale-out).
	<-g.closedCh

	g.wg.Wait()
	g.mu.Lock()
	err := g.firstErr
	g.mu.Unlock()
	return err
}

func (g *ProcessGroup) Closed() <-chan struct{} {
	if g == nil || g.closedCh == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return g.closedCh
}
