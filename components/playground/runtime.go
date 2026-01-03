package main

import (
	"io"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

var _ pgservice.Runtime = (*Playground)(nil)

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

func (p *Playground) SharedOptions() proc.SharedOptions {
	if p == nil || p.bootOptions == nil {
		return proc.SharedOptions{}
	}
	return p.bootOptions.ShOpt
}

func (p *Playground) DataDir() string {
	if p == nil {
		return ""
	}
	return p.dataDir
}

func (p *Playground) BootConfig(serviceID proc.ServiceID) (proc.Config, bool) {
	if p == nil || serviceID == "" || p.bootBaseConfigs == nil {
		return proc.Config{}, false
	}
	cfg, ok := p.bootBaseConfigs[serviceID]
	return cfg, ok
}

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

func (p *Playground) AddProc(serviceID proc.ServiceID, inst proc.Process) {
	panic("Playground.AddProc is controller-only; use controllerRuntime")
}

func (p *Playground) RemoveProc(serviceID proc.ServiceID, inst proc.Process) bool {
	panic("Playground.RemoveProc is controller-only; use controllerRuntime")
}

func (p *Playground) ExpectExitPID(pid int) {
	panic("Playground.ExpectExitPID is controller-only; use controllerRuntime")
}

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

func (p *Playground) EmitEvent(evt any) {
	if p == nil {
		return
	}
	p.emitEvent(evt)
}

func (p *Playground) TermWriter() io.Writer {
	return p.termWriter()
}

func (p *Playground) OnProcsChanged() {
	if p == nil {
		return
	}
	p.progressMu.Lock()
	p.procTitleCounts = p.buildProcTitleCounts()
	p.progressMu.Unlock()

	logIfErr(p.renderSDFile())
}
