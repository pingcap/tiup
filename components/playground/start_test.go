package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/repository"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/pingcap/tiup/pkg/utils"
)

type fakeComponentBinaryInstaller struct {
	binPath string

	updateSpecs [][]repository.ComponentSpec
	onUpdate    func(specs []repository.ComponentSpec) error
}

func (f *fakeComponentBinaryInstaller) BinaryPath(component string, v utils.Version) (string, error) {
	return f.binPath, nil
}

func (f *fakeComponentBinaryInstaller) UpdateComponents(specs []repository.ComponentSpec) error {
	f.updateSpecs = append(f.updateSpecs, specs)
	if f.onUpdate != nil {
		return f.onUpdate(specs)
	}
	return nil
}

func TestPrepareComponentBinaryWithInstaller_BinaryExistsSkipsUpdate(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")
	if err := os.WriteFile(binPath, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	inst := &fakeComponentBinaryInstaller{binPath: binPath}
	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != binPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 0 {
		t.Fatalf("unexpected UpdateComponents calls: %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_BinaryMissingForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")

	inst := &fakeComponentBinaryInstaller{
		binPath: binPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			if len(specs) != 1 {
				t.Fatalf("unexpected specs: %+v", specs)
			}
			if !specs[0].Force {
				t.Fatalf("expected Force=true, got %+v", specs[0])
			}
			if err := os.WriteFile(binPath, []byte("ok"), 0o755); err != nil {
				t.Fatalf("write bin: %v", err)
			}
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != binPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_AfterUpdateStillMissingReturnsError(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")

	inst := &fakeComponentBinaryInstaller{
		binPath: binPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			return nil
		},
	}

	_, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_MissingRequiredBinaryForService_ForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	baseBinPath := filepath.Join(dir, "tikv-server")
	if err := os.WriteFile(baseBinPath, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write tikv-server: %v", err)
	}

	requiredBinPath := filepath.Join(dir, "tikv-worker")

	inst := &fakeComponentBinaryInstaller{
		binPath: baseBinPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			if len(specs) != 1 {
				t.Fatalf("unexpected specs: %+v", specs)
			}
			if !specs[0].Force {
				t.Fatalf("expected Force=true, got %+v", specs[0])
			}
			if err := os.WriteFile(requiredBinPath, []byte("ok"), 0o755); err != nil {
				t.Fatalf("write tikv-worker: %v", err)
			}
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServiceTiKVWorker, "tikv", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != requiredBinPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_NGMonitoring_UsesSiblingBinary(t *testing.T) {
	dir := t.TempDir()
	baseBinPath := filepath.Join(dir, "prometheus")
	if err := os.WriteFile(baseBinPath, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write prometheus: %v", err)
	}

	requiredBinPath := filepath.Join(dir, "ng-monitoring-server")

	inst := &fakeComponentBinaryInstaller{
		binPath: baseBinPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			if len(specs) != 1 {
				t.Fatalf("unexpected specs: %+v", specs)
			}
			if !specs[0].Force {
				t.Fatalf("expected Force=true, got %+v", specs[0])
			}
			if err := os.WriteFile(requiredBinPath, []byte("ok"), 0o755); err != nil {
				t.Fatalf("write ng-monitoring-server: %v", err)
			}
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServiceNGMonitoring, "prometheus", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != requiredBinPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}

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

	readyCh, err := pg.startProcWithControllerState(context.Background(), &controllerState{}, inst)
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
