package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"syscall"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

type controllerEvent interface{}

type controllerState struct {
	booting bool
	booted  bool

	procs            map[proc.ServiceID][]proc.Process
	requiredServices map[proc.ServiceID]int
	criticalRunning  map[proc.ServiceID]int
	expectedExit     map[int]struct{}
	idAlloc          map[proc.ServiceID]int

	procByPID  map[int]*procRecord
	procByName map[string]*procRecord
}

type procExitedEvent struct {
	inst   proc.Process
	pid    int
	err    error
	respCh chan procExitDecision
}

type procExitDecision struct {
	expectedExit bool
}

type bootStateEvent struct {
	booting bool
	ackCh   chan struct{}
}

type bootedStateEvent struct {
	booted bool
	ackCh  chan struct{}
}

type setRequiredServicesEvent struct {
	required map[proc.ServiceID]int
	ackCh    chan struct{}
}

type procsByServiceRequest struct {
	serviceID proc.ServiceID
	respCh    chan []proc.Process
}

type procsSnapshotRequest struct {
	respCh chan map[proc.ServiceID][]proc.Process
}

type procRecordsSnapshotRequest struct {
	respCh chan []procRecordSnapshot
}

type bootedStateRequest struct {
	respCh chan bool
}

type addProcRequest struct {
	serviceID proc.ServiceID
	cfg       proc.Config
	respCh    chan addProcResponse
}

type addProcResponse struct {
	inst proc.Process
	err  error
}

type startProcRequest struct {
	ctx     context.Context
	inst    proc.Process
	preload *binaryPreloader
	respCh  chan startProcResponse
}

type startProcResponse struct {
	readyCh <-chan error
	err     error
}

type stopSignalEvent struct {
	sig syscall.Signal
}

type stopInternalEvent struct{}

type forceKillEvent struct{}

type commandRequest struct {
	cmd    *Command
	respCh chan commandResponse
}

type commandResponse struct {
	output []byte
	err    error
}

func (p *Playground) startController() {
	if p == nil {
		return
	}
	p.controllerOnce.Do(func() {
		// The controller lifetime is managed explicitly via p.controllerCancel.
		// Do not bind it to the boot context: Ctrl+C cancels boot to abort
		// downloads quickly, but shutdown still needs the controller-owned state
		// to terminate processes.
		controllerCtx, cancel := context.WithCancel(context.Background())
		p.controllerCancel = cancel
		p.cmdReqCh = make(chan commandRequest)
		p.evtCh = make(chan controllerEvent, 64)
		p.controllerDoneCh = make(chan struct{})
		go p.controllerLoop(controllerCtx)
	})
}

func (p *Playground) controllerLoop(ctx context.Context) {
	state := controllerState{
		procs:            make(map[proc.ServiceID][]proc.Process),
		requiredServices: make(map[proc.ServiceID]int),
		criticalRunning:  make(map[proc.ServiceID]int),
		expectedExit:     make(map[int]struct{}),
		idAlloc:          make(map[proc.ServiceID]int),
		procByPID:        make(map[int]*procRecord),
		procByName:       make(map[string]*procRecord),
	}
	defer func() {
		if p != nil && p.controllerDoneCh != nil {
			close(p.controllerDoneCh)
		}
	}()
	for {
		select {
		case req := <-p.cmdReqCh:
			var buf bytes.Buffer
			err := p.handleCommand(&state, req.cmd, &buf)
			req.respCh <- commandResponse{output: buf.Bytes(), err: err}
		case evt := <-p.evtCh:
			p.handleEvent(&state, evt)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Playground) handleEvent(state *controllerState, evt controllerEvent) {
	switch e := evt.(type) {
	case bootStateEvent:
		if state != nil {
			state.booting = e.booting
		}
		if e.ackCh != nil {
			close(e.ackCh)
		}
	case bootedStateEvent:
		if state != nil {
			state.booted = e.booted
		}
		if e.ackCh != nil {
			close(e.ackCh)
		}
	case setRequiredServicesEvent:
		if state != nil {
			state.requiredServices = make(map[proc.ServiceID]int, len(e.required))
			for k, v := range e.required {
				if k != "" && v > 0 {
					state.requiredServices[k] = v
				}
			}
			state.criticalRunning = make(map[proc.ServiceID]int)
		}
		if e.ackCh != nil {
			close(e.ackCh)
		}
	case stopSignalEvent:
		p.startShutdownWithControllerState(state, stopCauseSignal, e.sig)
	case stopInternalEvent:
		p.startShutdownWithControllerState(state, stopCauseInternal, 0)
	case forceKillEvent:
		p.forceKillShutdownWithControllerState(state)
	case procsByServiceRequest:
		var out []proc.Process
		if state != nil && state.procs != nil && e.serviceID != "" {
			out = append([]proc.Process(nil), state.procs[e.serviceID]...)
		}
		if e.respCh != nil {
			e.respCh <- out
			close(e.respCh)
		}
	case procsSnapshotRequest:
		out := make(map[proc.ServiceID][]proc.Process)
		if state != nil && state.procs != nil {
			for id, list := range state.procs {
				out[id] = append([]proc.Process(nil), list...)
			}
		}
		if e.respCh != nil {
			e.respCh <- out
			close(e.respCh)
		}
	case procRecordsSnapshotRequest:
		var out []procRecordSnapshot
		if state != nil {
			out = state.snapshotProcRecords()
		}
		if e.respCh != nil {
			e.respCh <- out
			close(e.respCh)
		}
	case bootedStateRequest:
		booted := false
		if state != nil {
			booted = state.booted
		}
		if e.respCh != nil {
			e.respCh <- booted
			close(e.respCh)
		}
	case addProcRequest:
		var (
			inst proc.Process
			err  error
		)
		if p != nil {
			inst, err = p.addProcInController(state, e.serviceID, e.cfg)
		}
		if e.respCh != nil {
			e.respCh <- addProcResponse{inst: inst, err: err}
			close(e.respCh)
		}
	case startProcRequest:
		var (
			readyCh <-chan error
			err     error
		)
		if p != nil {
			readyCh, err = p.startProc(state, e.ctx, e.inst, e.preload)
		}
		if e.respCh != nil {
			e.respCh <- startProcResponse{readyCh: readyCh, err: err}
			close(e.respCh)
		}
	case procExitedEvent:
		booting := false
		if state != nil {
			booting = state.booting
		}
		dec := p.handleProcExited(state, e.inst, e.pid, e.err, booting)
		if e.respCh != nil {
			e.respCh <- dec
			close(e.respCh)
		}
	default:
		if se, ok := evt.(pgservice.Event); ok && se != nil {
			se.Handle(controllerRuntime{pg: p, state: state})
		}
	}
}

func (p *Playground) emitEvent(evt controllerEvent) {
	if p == nil || p.evtCh == nil {
		return
	}
	if p.controllerDoneCh == nil {
		select {
		case p.evtCh <- evt:
		default:
		}
		return
	}
	select {
	case p.evtCh <- evt:
	case <-p.controllerDoneCh:
	}
}

func (p *Playground) doCommand(ctx context.Context, cmd *Command) ([]byte, error) {
	if p == nil {
		return nil, context.Canceled
	}
	if cmd == nil {
		return nil, context.Canceled
	}
	if p.cmdReqCh == nil {
		return nil, context.Canceled
	}

	respCh := make(chan commandResponse, 1)
	req := commandRequest{
		cmd:    cmd,
		respCh: respCh,
	}

	select {
	case p.cmdReqCh <- req:
	case <-p.controllerDoneCh:
		return nil, fmt.Errorf("playground is stopping")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case resp := <-respCh:
		return resp.output, resp.err
	case <-p.controllerDoneCh:
		return nil, fmt.Errorf("playground is stopping")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Playground) handleCommand(state *controllerState, cmd *Command, w io.Writer) error {
	if cmd == nil {
		return fmt.Errorf("command is nil")
	}

	// Reject commands while stopping to keep lifecycle predictable.
	if p.Stopping() {
		return fmt.Errorf("playground is stopping")
	}

	switch cmd.Type {
	case DisplayCommandType:
		verbose := false
		jsonOut := false
		if cmd.Display != nil {
			verbose = cmd.Display.Verbose
			jsonOut = cmd.Display.JSON
		}
		return p.handleDisplay(state, w, verbose, jsonOut)
	case ScaleInCommandType:
		if cmd.ScaleIn == nil {
			return fmt.Errorf("missing scale_in request")
		}
		return p.handleScaleIn(state, w, cmd.ScaleIn)
	case ScaleOutCommandType:
		return p.handleScaleOut(state, w, cmd.ScaleOut)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

func buildProcTitleCountsFromProcs(procs map[proc.ServiceID][]proc.Process) map[string]int {
	counts := make(map[string]int)
	for _, list := range procs {
		for _, inst := range list {
			if inst == nil {
				continue
			}
			counts[procTitle(inst)]++
		}
	}
	return counts
}

func (p *Playground) onProcsChangedInController(state *controllerState) {
	if p == nil || state == nil {
		return
	}
	p.progressMu.Lock()
	p.procTitleCounts = buildProcTitleCountsFromProcs(state.procs)
	p.progressMu.Unlock()

	logIfErr(p.renderSDFileInController(state))
}

func (p *Playground) renderSDFileInController(state *controllerState) error {
	if p == nil || state == nil {
		return nil
	}

	proms := state.procs[proc.ServicePrometheus]
	if len(proms) == 0 || proms[0] == nil {
		return nil
	}
	prom, ok := proms[0].(*proc.PrometheusInstance)
	if !ok || prom == nil {
		return nil
	}
	return renderPrometheusSDFile(prom, state.walkProcs)
}

// handleProcStarted runs in the controller goroutine.
func (p *Playground) handleProcStarted(state *controllerState, inst proc.Process) {
	if p == nil || inst == nil {
		return
	}

	info := inst.Info()
	if info == nil {
		return
	}

	if state != nil {
		state.upsertProcRecord(inst)
	}
	serviceID := info.Service
	min := 0
	if state != nil && state.requiredServices != nil {
		min = state.requiredServices[serviceID]
	}
	if min <= 0 {
		return
	}
	if state == nil {
		return
	}
	if state.criticalRunning == nil {
		state.criticalRunning = make(map[proc.ServiceID]int)
	}
	state.criticalRunning[serviceID]++
}

// handleProcExited runs in the controller goroutine.
func (p *Playground) handleProcExited(state *controllerState, inst proc.Process, pid int, err error, booting bool) procExitDecision {
	if p == nil || inst == nil {
		return procExitDecision{}
	}

	info := inst.Info()
	if info == nil {
		return procExitDecision{}
	}

	expectedExit := false
	if pid > 0 && state != nil && state.expectedExit != nil {
		if _, ok := state.expectedExit[pid]; ok {
			expectedExit = true
			delete(state.expectedExit, pid)
		}
	}

	if state != nil {
		state.deleteProcRecord(pid, info.Name())
	}

	serviceID := info.Service
	min := 0
	if state != nil && state.requiredServices != nil {
		min = state.requiredServices[serviceID]
	}
	if min > 0 && state != nil && state.criticalRunning != nil {
		if state.criticalRunning[serviceID] > 0 {
			state.criticalRunning[serviceID]--
		}
	}
	remaining := 0
	if state != nil && state.criticalRunning != nil {
		remaining = state.criticalRunning[serviceID]
	}

	triggerAutoStop := min > 0 && remaining < min && !expectedExit

	if !expectedExit {
		exitErr := err
		if triggerAutoStop && exitErr == nil {
			exitErr = fmt.Errorf("%s exited", info.Name())
		}
		if exitErr != nil {
			p.markStartingTaskError(inst, "", exitErr)
		}
	}

	// During shutdown, we manage user-facing output via the shutdown progress
	// group. Avoid printing per-process quit lines, which would otherwise
	// interleave with multi-line progress rendering.
	if p.Stopping() {
		return procExitDecision{expectedExit: expectedExit}
	}

	if !expectedExit {
		if err != nil {
			p.printProcExitError(inst, err)
		} else {
			fmt.Fprintf(p.termWriter(), "%s quit\n", p.shutdownProcTitle(inst))
		}
	}

	if triggerAutoStop {
		exitErr := err
		if exitErr == nil {
			exitErr = fmt.Errorf("%s exited", info.Name())
		}
		bootErr := renderedError{err: fmt.Errorf("%s exited: %w", procDisplayName(inst, true), exitErr)}
		if booting {
			p.cancelBootWithCause(bootErr)
		}
		// Start shutdown after printing the root cause above, so subsequent
		// process exits won't interleave with user-facing output.
		p.startShutdownWithControllerState(state, stopCauseInternal, 0)
	}

	return procExitDecision{expectedExit: expectedExit}
}

func (p *Playground) setControllerBooting(ctx context.Context, booting bool) {
	if p == nil || p.evtCh == nil {
		return
	}

	ackCh := make(chan struct{})
	p.emitEvent(bootStateEvent{booting: booting, ackCh: ackCh})

	if ackCh == nil {
		return
	}
	if ctx == nil {
		<-ackCh
		return
	}
	select {
	case <-ackCh:
	case <-ctx.Done():
	case <-p.controllerDoneCh:
	}
}

func (p *Playground) setControllerRequiredServices(ctx context.Context, required map[proc.ServiceID]int) {
	if p == nil || p.evtCh == nil {
		return
	}

	ackCh := make(chan struct{})
	p.emitEvent(setRequiredServicesEvent{required: required, ackCh: ackCh})

	if ackCh == nil {
		return
	}
	if ctx == nil {
		<-ackCh
		return
	}
	select {
	case <-ackCh:
	case <-ctx.Done():
	case <-p.controllerDoneCh:
	}
}

func (p *Playground) setControllerBooted(ctx context.Context, booted bool) {
	if p == nil || p.evtCh == nil {
		return
	}

	ackCh := make(chan struct{})
	p.emitEvent(bootedStateEvent{booted: booted, ackCh: ackCh})

	if ackCh == nil {
		return
	}
	if ctx == nil {
		<-ackCh
		return
	}
	select {
	case <-ackCh:
	case <-ctx.Done():
	case <-p.controllerDoneCh:
	}
}

func (p *Playground) requestStopSignal(sig syscall.Signal) {
	if p == nil {
		return
	}
	if p.evtCh == nil {
		p.startShutdown(stopCauseSignal, sig)
		return
	}
	if p.controllerDoneCh != nil {
		select {
		case <-p.controllerDoneCh:
			p.startShutdown(stopCauseSignal, sig)
			return
		default:
		}
	}
	p.emitEvent(stopSignalEvent{sig: sig})
}

func (p *Playground) requestStopInternal() {
	if p == nil {
		return
	}
	if p.evtCh == nil {
		p.startShutdown(stopCauseInternal, 0)
		return
	}
	if p.controllerDoneCh != nil {
		select {
		case <-p.controllerDoneCh:
			p.startShutdown(stopCauseInternal, 0)
			return
		default:
		}
	}
	p.emitEvent(stopInternalEvent{})
}

func (p *Playground) requestForceKill() {
	if p == nil {
		return
	}
	if p.evtCh == nil {
		p.forceKillShutdown()
		return
	}
	if p.controllerDoneCh != nil {
		select {
		case <-p.controllerDoneCh:
			p.forceKillShutdown()
			return
		default:
		}
	}
	p.emitEvent(forceKillEvent{})
}
