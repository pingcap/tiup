package service

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
)

type fakeRuntime struct {
	procs          map[proc.ServiceID][]proc.Process
	out            *bytes.Buffer
	procsChanged   int
	expectedExit   []int
	stopping       bool
}

func (rt *fakeRuntime) Booted() bool { return true }

func (rt *fakeRuntime) SharedOptions() proc.SharedOptions { return proc.SharedOptions{} }

func (rt *fakeRuntime) DataDir() string { return "" }

func (rt *fakeRuntime) BootConfig(serviceID proc.ServiceID) (proc.Config, bool) { return proc.Config{}, false }

func (rt *fakeRuntime) Procs(serviceID proc.ServiceID) []proc.Process {
	if rt == nil || rt.procs == nil || serviceID == "" {
		return nil
	}
	return append([]proc.Process(nil), rt.procs[serviceID]...)
}

func (rt *fakeRuntime) AddProc(serviceID proc.ServiceID, inst proc.Process) {
	if rt == nil || serviceID == "" || inst == nil {
		return
	}
	if rt.procs == nil {
		rt.procs = make(map[proc.ServiceID][]proc.Process)
	}
	rt.procs[serviceID] = append(rt.procs[serviceID], inst)
}

func (rt *fakeRuntime) RemoveProc(serviceID proc.ServiceID, inst proc.Process) bool {
	if rt == nil || rt.procs == nil || serviceID == "" || inst == nil {
		return false
	}
	list := rt.procs[serviceID]
	for i := 0; i < len(list); i++ {
		if list[i] != inst {
			continue
		}
		rt.procs[serviceID] = append(list[:i], list[i+1:]...)
		return true
	}
	return false
}

func (rt *fakeRuntime) ExpectExitPID(pid int) {
	if pid <= 0 {
		return
	}
	rt.expectedExit = append(rt.expectedExit, pid)
}

func (rt *fakeRuntime) Stopping() bool { return rt != nil && rt.stopping }

func (rt *fakeRuntime) EmitEvent(evt any) {
	if rt == nil {
		return
	}
	if e, ok := evt.(Event); ok && e != nil {
		e.Handle(rt)
	}
}

func (rt *fakeRuntime) TermWriter() io.Writer {
	if rt == nil {
		return io.Discard
	}
	if rt.out == nil {
		rt.out = &bytes.Buffer{}
	}
	return rt.out
}

func (rt *fakeRuntime) OnProcsChanged() {
	if rt == nil {
		return
	}
	rt.procsChanged++
}

type fakeProcess struct {
	info proc.ProcessInfo
}

func (p *fakeProcess) Info() *proc.ProcessInfo { return &p.info }

func (p *fakeProcess) Prepare(context.Context) error { return nil }

func (p *fakeProcess) LogFile() string { return "" }

func TestWatchAsyncScaleInStopEmitsEvent(t *testing.T) {
	rt := &fakeRuntime{out: &bytes.Buffer{}}
	inst := &fakeProcess{
		info: proc.ProcessInfo{
			ID:      0,
			Service: proc.ServiceTiKV,
		},
	}
	rt.AddProc(proc.ServiceTiKV, inst)

	watchAsyncScaleInStop(rt, time.Millisecond, func() (done bool, err error) {
		return true, nil
	}, asyncScaleInStopEvent{
		serviceID:   proc.ServiceTiKV,
		inst:        inst,
		stopMessage: "stop tombstone tikv 127.0.0.1:20160",
	})

	if got := rt.Procs(proc.ServiceTiKV); len(got) != 0 {
		t.Fatalf("expected instance to be removed, got %v", got)
	}
	if rt.procsChanged != 1 {
		t.Fatalf("expected OnProcsChanged called once, got %d", rt.procsChanged)
	}
	if rt.out == nil || !bytes.Contains(rt.out.Bytes(), []byte("stop tombstone tikv")) {
		t.Fatalf("expected stop message written, got %q", rt.out.String())
	}
}

