package main

import (
	"bytes"
	"context"
	"fmt"
	"syscall"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

type controllerEvent interface{}

type controllerState struct {
	booting bool
}

type procStartedEvent struct {
	inst proc.Process
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

func (p *Playground) startController(ctx context.Context) {
	if p == nil {
		return
	}
	p.controllerOnce.Do(func() {
		p.cmdReqCh = make(chan commandRequest)
		p.evtCh = make(chan controllerEvent, 64)
		p.controllerDoneCh = make(chan struct{})
		go p.controllerLoop(ctx)
	})
}

func (p *Playground) controllerLoop(ctx context.Context) {
	state := controllerState{}
	defer func() {
		if p != nil && p.controllerDoneCh != nil {
			close(p.controllerDoneCh)
		}
	}()
	for {
		select {
		case req := <-p.cmdReqCh:
			var buf bytes.Buffer
			err := p.handleCommand(req.cmd, &buf)
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
	case stopSignalEvent:
		p.startShutdown(stopCauseSignal, e.sig)
	case stopInternalEvent:
		p.startShutdown(stopCauseInternal, 0)
	case forceKillEvent:
		p.forceKillShutdown()
	case procStartedEvent:
		p.handleProcStarted(e.inst)
	case procExitedEvent:
		booting := false
		if state != nil {
			booting = state.booting
		}
		dec := p.handleProcExited(e.inst, e.pid, e.err, booting)
		if e.respCh != nil {
			e.respCh <- dec
			close(e.respCh)
		}
	default:
		if se, ok := evt.(pgservice.Event); ok && se != nil {
			se.Handle(p)
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

// handleProcStarted runs in the controller goroutine.
func (p *Playground) handleProcStarted(inst proc.Process) {
	if p == nil || inst == nil {
		return
	}

	info := inst.Info()
	if info == nil {
		return
	}
	serviceID := info.Service
	min := p.requiredServices[serviceID]
	if min <= 0 {
		return
	}
	if p.criticalRunning == nil {
		p.criticalRunning = make(map[proc.ServiceID]int)
	}
	p.criticalRunning[serviceID]++
}

// handleProcExited runs in the controller goroutine.
func (p *Playground) handleProcExited(inst proc.Process, pid int, err error, booting bool) procExitDecision {
	if p == nil || inst == nil {
		return procExitDecision{}
	}

	info := inst.Info()
	if info == nil {
		return procExitDecision{}
	}

	expectedExit := false
	if pid > 0 && p.expectedExit != nil {
		if _, ok := p.expectedExit[pid]; ok {
			expectedExit = true
			delete(p.expectedExit, pid)
		}
	}

	serviceID := info.Service
	min := p.requiredServices[serviceID]
	if min > 0 && p.criticalRunning != nil {
		if p.criticalRunning[serviceID] > 0 {
			p.criticalRunning[serviceID]--
		}
	}
	remaining := 0
	if p.criticalRunning != nil {
		remaining = p.criticalRunning[serviceID]
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
		p.startShutdown(stopCauseInternal, 0)
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
