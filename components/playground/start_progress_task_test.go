package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
)

type blockingProgressTask struct {
	blockCh <-chan struct{}

	doneOnce sync.Once
	doneCh   chan struct{}
}

func newBlockingProgressTask(blockCh <-chan struct{}) *blockingProgressTask {
	return &blockingProgressTask{blockCh: blockCh, doneCh: make(chan struct{})}
}

func (t *blockingProgressTask) SetMeta(meta string)  { <-t.blockCh }
func (t *blockingProgressTask) Start()               { <-t.blockCh }
func (t *blockingProgressTask) Done()                { t.doneOnce.Do(func() { close(t.doneCh) }) }
func (t *blockingProgressTask) Error(msg string)     {}
func (t *blockingProgressTask) Cancel(reason string) {}

type fakeOSProcess struct {
	startedOnce sync.Once
	startedCh   chan struct{}
}

func newFakeOSProcess() *fakeOSProcess {
	return &fakeOSProcess{startedCh: make(chan struct{})}
}

func (p *fakeOSProcess) Start() error {
	p.startedOnce.Do(func() { close(p.startedCh) })
	return nil
}

func (p *fakeOSProcess) Wait() error                      { return nil }
func (p *fakeOSProcess) Pid() int                         { return 123 }
func (p *fakeOSProcess) Uptime() string                   { return "" }
func (p *fakeOSProcess) SetOutputFile(fname string) error { return nil }
func (p *fakeOSProcess) Cmd() *exec.Cmd                   { return &exec.Cmd{} }

type fakeProcess struct {
	info    *proc.ProcessInfo
	osProc  proc.OSProcess
	logFile string
}

func (p *fakeProcess) Info() *proc.ProcessInfo { return p.info }

func (p *fakeProcess) Prepare(ctx context.Context) error {
	p.info.Proc = p.osProc
	return nil
}

func (p *fakeProcess) LogFile() string { return p.logFile }

func TestStartProcWithControllerState_DoesNotBlockOnProgressTask(t *testing.T) {
	blockCh := make(chan struct{})
	task := newBlockingProgressTask(blockCh)

	osProc := newFakeOSProcess()
	info := &proc.ProcessInfo{
		Service:     proc.ServiceTiDB,
		ID:          0,
		BinPath:     "/tmp/tidb-server",
		UserBinPath: "/tmp/tidb-server",
	}
	inst := &fakeProcess{
		info:    info,
		osProc:  osProc,
		logFile: filepath.Join(t.TempDir(), "tidb.log"),
	}

	pg := NewPlayground(t.TempDir(), 0)
	pg.startingGroup = &progressv2.Group{}
	pg.startingTasks = map[string]progressTask{
		info.Name(): task,
	}
	pg.controllerDoneCh = make(chan struct{})
	close(pg.controllerDoneCh)

	readyCh, err := pg.startProcWithControllerState(&controllerState{}, context.Background(), inst)
	if err != nil {
		t.Fatalf("startProcWithControllerState: %v", err)
	}
	select {
	case <-readyCh:
	default:
		t.Fatalf("expected readyCh to be closed for non-ReadyWaiter instance")
	}

	select {
	case <-osProc.startedCh:
	case <-time.After(time.Second):
		t.Fatalf("process Start() did not run while progress task is blocked")
	}

	select {
	case <-task.doneCh:
		t.Fatalf("unexpected task.Done() before progress task is started")
	default:
	}

	close(blockCh)

	select {
	case <-task.doneCh:
	case <-time.After(time.Second):
		t.Fatalf("task.Done() not invoked after progress task unblocked")
	}
}
