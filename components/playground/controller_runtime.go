package main

import (
	"io"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

var _ pgservice.Runtime = controllerRuntime{}

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
		return nil
	}
	return rt.pg.termWriter()
}

func (rt controllerRuntime) OnProcsChanged() {
	if rt.pg == nil || rt.state == nil {
		return
	}
	rt.pg.onProcsChangedInController(rt.state)
}
