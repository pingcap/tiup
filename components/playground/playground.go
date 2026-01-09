// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
)

// Playground represent the playground of a cluster.
type Playground struct {
	dataDir         string
	deleteWhenExit  bool
	bootOptions     *BootOptions
	bootBaseConfigs map[proc.ServiceID]proc.Config
	port            int

	// shutdownProcRecords snapshots controller-owned proc records at the moment
	// shutdown starts. It lets termination logic work after the controller loop
	// is canceled (no more events/commands).
	shutdownProcRecords []procRecordSnapshot

	bootCancel context.CancelCauseFunc

	shutdownOnce  sync.Once
	stoppingCh    chan struct{}
	interruptedCh chan struct{}
	processGroup  *ProcessGroup

	ui            *progressv2.UI
	startingGroup *progressv2.Group
	downloadGroup *progressv2.Group
	shutdownGroup *progressv2.Group

	// progressMu protects UI-related state that is accessed from multiple
	// goroutines (boot flow, stop handling, instance waiters), including:
	// - progress groups/tasks (download/starting/shutdown)
	// - procTitleCounts used for user-facing titles
	progressMu sync.Mutex

	// startingTasks holds one progress task per instance during boot, keyed by
	// inst.Info().Name(). It lets "Starting instances" show the
	// full instance list from the beginning (including components that start later,
	// like TiFlash).
	startingTasks map[string]progressTask

	// procTitleCounts is computed once during boot to decide whether to display
	// an instance index in user-facing output (e.g. "TiKV 0").
	//
	// It is refreshed when the instance set changes (boot, scale-out, scale-in).
	procTitleCounts map[string]int

	terminateDoneCh   chan struct{}
	terminateDoneOnce sync.Once

	controllerOnce   sync.Once
	controllerCancel context.CancelFunc
	cmdReqCh         chan commandRequest
	evtCh            chan controllerEvent
	controllerDoneCh chan struct{}
}

// NewPlayground create a Playground proc.
func NewPlayground(dataDir string, port int) *Playground {
	return &Playground{
		dataDir:         dataDir,
		port:            port,
		stoppingCh:      make(chan struct{}),
		interruptedCh:   make(chan struct{}),
		terminateDoneCh: make(chan struct{}),
		processGroup:    NewProcessGroup(),
	}
}

func (p *Playground) terminalWriter() io.Writer {
	if p != nil && p.ui != nil {
		return p.ui.Writer()
	}
	return tuiv2output.Stdout.Get()
}

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

// NewProcessGroup creates a new ProcessGroup.
func NewProcessGroup() *ProcessGroup {
	return &ProcessGroup{
		closedCh: make(chan struct{}),
	}
}

// Add adds a new goroutine to the group.
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

// Close stops accepting new goroutines and unblocks Wait.
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

// Wait blocks until Close is called and all added goroutines finish.
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

// Closed returns a channel that is closed when the group is closed.
func (g *ProcessGroup) Closed() <-chan struct{} {
	if g == nil || g.closedCh == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return g.closedCh
}

var _ pgservice.Runtime = (*Playground)(nil)

// Booted reports whether the playground finished booting successfully.
func (p *Playground) Booted() bool {
	if p == nil || p.evtCh == nil {
		return false
	}
	respCh := make(chan bool, 1)
	p.emitEvent(bootedStateRequest{respCh: respCh})
	select {
	case v := <-respCh:
		return v
	case <-p.controllerDoneCh:
		return false
	}
}

// SharedOptions returns the boot-time shared options.
func (p *Playground) SharedOptions() proc.SharedOptions {
	if p == nil || p.bootOptions == nil {
		return proc.SharedOptions{}
	}
	return p.bootOptions.ShOpt
}

// DataDir returns the playground data directory.
func (p *Playground) DataDir() string {
	if p == nil {
		return ""
	}
	return p.dataDir
}

// BootConfig returns the boot-time base config for the given service.
func (p *Playground) BootConfig(serviceID proc.ServiceID) (proc.Config, bool) {
	if p == nil || serviceID == "" || p.bootBaseConfigs == nil {
		return proc.Config{}, false
	}
	cfg, ok := p.bootBaseConfigs[serviceID]
	return cfg, ok
}

// Procs returns a snapshot of instances currently tracked for the service.
func (p *Playground) Procs(serviceID proc.ServiceID) []proc.Process {
	if p == nil || serviceID == "" || p.evtCh == nil {
		return nil
	}
	respCh := make(chan []proc.Process, 1)
	p.emitEvent(procsByServiceRequest{serviceID: serviceID, respCh: respCh})
	select {
	case list := <-respCh:
		return list
	case <-p.controllerDoneCh:
		return nil
	}
}

// Stopping reports whether playground shutdown has started.
func (p *Playground) Stopping() bool {
	if p == nil {
		return true
	}
	ch := p.stoppingCh
	if ch == nil {
		return true
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// EmitEvent enqueues an event for the controller goroutine.
func (p *Playground) EmitEvent(evt any) {
	if p == nil {
		return
	}
	p.emitEvent(evt)
}

// TermWriter returns the writer used for user-facing output.
func (p *Playground) TermWriter() io.Writer {
	return p.terminalWriter()
}

// OnProcsChanged refreshes any derived output state after the proc set changes.
func (p *Playground) OnProcsChanged() {
	if p == nil {
		return
	}
	p.progressMu.Lock()
	p.procTitleCounts = p.buildProcTitleCounts()
	p.progressMu.Unlock()

	logIfErr(p.renderSDFile())
}

var _ pgservice.Runtime = controllerRuntime{}
var _ pgservice.ControllerRuntime = controllerRuntime{}

type controllerRuntime struct {
	pg    *Playground
	state *controllerState
}

func (rt controllerRuntime) Booted() bool {
	if rt.state == nil {
		return false
	}
	return rt.state.booted
}

func (rt controllerRuntime) SharedOptions() proc.SharedOptions {
	if rt.pg == nil || rt.pg.bootOptions == nil {
		return proc.SharedOptions{}
	}
	return rt.pg.bootOptions.ShOpt
}

func (rt controllerRuntime) DataDir() string {
	if rt.pg == nil {
		return ""
	}
	return rt.pg.dataDir
}

func (rt controllerRuntime) BootConfig(serviceID proc.ServiceID) (proc.Config, bool) {
	if rt.pg == nil {
		return proc.Config{}, false
	}
	return rt.pg.BootConfig(serviceID)
}

func (rt controllerRuntime) Procs(serviceID proc.ServiceID) []proc.Process {
	if rt.state == nil || serviceID == "" {
		return nil
	}
	return append([]proc.Process(nil), rt.state.procs[serviceID]...)
}

func (rt controllerRuntime) AddProc(serviceID proc.ServiceID, inst proc.Process) {
	if rt.state == nil {
		return
	}
	rt.state.appendProc(serviceID, inst)
}

func (rt controllerRuntime) RemoveProc(serviceID proc.ServiceID, inst proc.Process) bool {
	if rt.state == nil {
		return false
	}
	return rt.state.removeProc(serviceID, inst)
}

func (rt controllerRuntime) ExpectExitPID(pid int) {
	if rt.state == nil || pid <= 0 {
		return
	}
	if rt.state.expectedExit == nil {
		rt.state.expectedExit = make(map[int]struct{})
	}
	rt.state.expectedExit[pid] = struct{}{}
}

func (rt controllerRuntime) Stopping() bool {
	if rt.pg == nil {
		return true
	}
	return rt.pg.Stopping()
}

func (rt controllerRuntime) EmitEvent(evt any) {
	if rt.pg == nil {
		return
	}
	rt.pg.emitEvent(evt)
}

func (rt controllerRuntime) TermWriter() io.Writer {
	if rt.pg == nil {
		return tuiv2output.Stdout.Get()
	}
	return rt.pg.terminalWriter()
}

func (rt controllerRuntime) OnProcsChanged() {
	if rt.pg == nil || rt.state == nil {
		return
	}
	rt.pg.onProcsChangedInController(rt.state)
}
