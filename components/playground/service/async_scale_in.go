package service

import (
	"fmt"
	"syscall"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
)

type asyncScaleInStopEvent struct {
	serviceID   proc.ServiceID
	inst        proc.Process
	stopMessage string
}

func (e asyncScaleInStopEvent) Handle(rt Runtime) {
	if rt == nil || e.inst == nil {
		return
	}

	if !rt.RemoveProc(e.serviceID, e.inst) {
		return
	}

	if e.stopMessage != "" {
		fmt.Fprintln(rt.TermWriter(), e.stopMessage)
	}

	pid := 0
	if info := e.inst.Info(); info != nil && info.Proc != nil {
		pid = info.Proc.Pid()
	}
	rt.ExpectExitPID(pid)
	if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGQUIT); err != nil {
			fmt.Fprintln(rt.TermWriter(), err)
		}
	}

	rt.OnProcsChanged()
}

func watchAsyncScaleInStop(rt Runtime, interval time.Duration, probe func() (done bool, err error), evt asyncScaleInStopEvent) {
	if rt == nil || probe == nil {
		return
	}

	pollUntil(rt, interval, 0, probe, func() {
		rt.EmitEvent(evt)
	})
}

