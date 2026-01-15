package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

type fakeComponentBinaryInstaller struct {
	binPath    string
	binPathErr error

	updateSpecs [][]repository.ComponentSpec
	onUpdate    func(specs []repository.ComponentSpec) error
}

func (f *fakeComponentBinaryInstaller) BinaryPath(component string, v utils.Version) (string, error) {
	if f.binPathErr != nil {
		return "", f.binPathErr
	}
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
	require.NoError(t, os.WriteFile(binPath, []byte("ok"), 0o755))

	inst := &fakeComponentBinaryInstaller{binPath: binPath}
	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	require.NoError(t, err)
	require.Equal(t, binPath, out)
	require.Empty(t, inst.updateSpecs)
}

func TestPrepareComponentBinaryWithInstaller_ForcePullForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")
	require.NoError(t, os.WriteFile(binPath, []byte("ok"), 0o755))

	inst := &fakeComponentBinaryInstaller{
		binPath: binPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			require.Len(t, specs, 1)
			require.True(t, specs[0].Force)
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), true)
	require.NoError(t, err)
	require.Equal(t, binPath, out)
	require.Len(t, inst.updateSpecs, 1)
}

func TestPrepareComponentBinaryWithInstaller_BinaryMissingForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")

	inst := &fakeComponentBinaryInstaller{
		binPath: binPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			require.Len(t, specs, 1)
			require.True(t, specs[0].Force)
			require.NoError(t, os.WriteFile(binPath, []byte("ok"), 0o755))
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	require.NoError(t, err)
	require.Equal(t, binPath, out)
	require.Len(t, inst.updateSpecs, 1)
}

func TestPrepareComponentBinaryWithInstaller_BinaryPathErrorStillUpdates(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")

	inst := &fakeComponentBinaryInstaller{binPathErr: errors.New("boom")}
	inst.onUpdate = func(specs []repository.ComponentSpec) error {
		require.Len(t, specs, 1)
		require.True(t, specs[0].Force)
		require.NoError(t, os.WriteFile(binPath, []byte("ok"), 0o755))
		inst.binPathErr = nil
		inst.binPath = binPath
		return nil
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	require.NoError(t, err)
	require.Equal(t, binPath, out)
	require.Len(t, inst.updateSpecs, 1)
}

func TestPrepareComponentBinaryWithInstaller_UpdateComponentsErrorReturnsError(t *testing.T) {
	inst := &fakeComponentBinaryInstaller{
		binPath: "",
		onUpdate: func(specs []repository.ComponentSpec) error {
			return errors.New("update failed")
		},
	}

	_, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	require.EqualError(t, err, "update failed")
	require.Len(t, inst.updateSpecs, 1)
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
	require.Error(t, err)
	require.Len(t, inst.updateSpecs, 1)
}

func TestPrepareComponentBinaryWithInstaller_MissingRequiredBinaryForService_ForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	baseBinPath := filepath.Join(dir, "tikv-server")
	require.NoError(t, os.WriteFile(baseBinPath, []byte("ok"), 0o755))

	requiredBinPath := filepath.Join(dir, "tikv-worker")

	inst := &fakeComponentBinaryInstaller{
		binPath: baseBinPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			require.Len(t, specs, 1)
			require.True(t, specs[0].Force)
			require.NoError(t, os.WriteFile(requiredBinPath, []byte("ok"), 0o755))
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServiceTiKVWorker, "tikv", utils.Version("v1.0.0"), false)
	require.NoError(t, err)
	require.Equal(t, requiredBinPath, out)
	require.Len(t, inst.updateSpecs, 1)
}

func TestPrepareComponentBinaryWithInstaller_NGMonitoring_UsesSiblingBinary(t *testing.T) {
	dir := t.TempDir()
	baseBinPath := filepath.Join(dir, "prometheus")
	require.NoError(t, os.WriteFile(baseBinPath, []byte("ok"), 0o755))

	requiredBinPath := filepath.Join(dir, "ng-monitoring-server")

	inst := &fakeComponentBinaryInstaller{
		binPath: baseBinPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			require.Len(t, specs, 1)
			require.True(t, specs[0].Force)
			require.NoError(t, os.WriteFile(requiredBinPath, []byte("ok"), 0o755))
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServiceNGMonitoring, "prometheus", utils.Version("v1.0.0"), false)
	require.NoError(t, err)
	require.Equal(t, requiredBinPath, out)
	require.Len(t, inst.updateSpecs, 1)
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

func TestStartProc_UserBinPath_DoesNotDependOnGlobalEnv(t *testing.T) {
	oldEnv := environment.GlobalEnv()
	environment.SetGlobalEnv(nil)
	defer environment.SetGlobalEnv(oldEnv)

	osProc := newFakeOSProcess()
	info := &proc.ProcessInfo{
		Service:     proc.ServiceTiDB,
		ID:          0,
		UserBinPath: "/tmp/tidb-server",
	}
	inst := &fakeProcess{
		info:    info,
		osProc:  osProc,
		logFile: filepath.Join(t.TempDir(), "tidb.log"),
	}

	pg := NewPlayground(t.TempDir(), 0)
	pg.controllerDoneCh = make(chan struct{})
	close(pg.controllerDoneCh)

	readyCh, err := pg.startProc(context.Background(), &controllerState{}, inst)
	require.NoError(t, err)
	require.Equal(t, info.UserBinPath, info.BinPath)
	require.Equal(t, utils.Version(utils.LatestVersionAlias), info.Version)

	select {
	case <-readyCh:
	default:
		require.FailNow(t, "expected readyCh to be closed for non-ReadyWaiter instance")
	}
}

func TestStartProc_UserBinPath_PreservesPlannedVersion(t *testing.T) {
	oldEnv := environment.GlobalEnv()
	environment.SetGlobalEnv(nil)
	defer environment.SetGlobalEnv(oldEnv)

	osProc := newFakeOSProcess()
	info := &proc.ProcessInfo{
		Service:     proc.ServiceTiDB,
		ID:          0,
		UserBinPath: "/tmp/tidb-server",
		Version:     utils.Version("v7.5.0"),
	}
	inst := &fakeProcess{
		info:    info,
		osProc:  osProc,
		logFile: filepath.Join(t.TempDir(), "tidb.log"),
	}

	pg := NewPlayground(t.TempDir(), 0)
	pg.controllerDoneCh = make(chan struct{})
	close(pg.controllerDoneCh)

	_, err := pg.startProc(context.Background(), &controllerState{}, inst)
	require.NoError(t, err)
	require.Equal(t, info.UserBinPath, info.BinPath)
	require.Equal(t, utils.Version("v7.5.0"), info.Version)
}

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
	require.NoError(t, err)
	select {
	case <-readyCh:
	default:
		require.FailNow(t, "expected readyCh to be closed for non-ReadyWaiter instance")
	}

	select {
	case <-osProc.startedCh:
	case <-time.After(time.Second):
		require.FailNow(t, "process Start() did not run while progress task is blocked")
	}

	select {
	case <-task.doneCh:
		require.FailNow(t, "unexpected task.Done() before progress task is started")
	default:
	}

	close(blockCh)

	select {
	case <-task.doneCh:
	case <-time.After(time.Second):
		require.FailNow(t, "task.Done() not invoked after progress task unblocked")
	}
}

func TestWaitBootStartingTasksSettled_EnsuresTaskDoneBeforePrintLines(t *testing.T) {
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out.log"))
	require.NoError(t, err)

	var eventBuf bytes.Buffer
	ui := progressv2.New(progressv2.Options{
		Mode:     progressv2.ModePlain,
		Out:      out,
		EventLog: &eventBuf,
	})

	pg := NewPlayground(dir, 0)
	pg.ui = ui
	pg.startingGroup = ui.Group("Start instances")

	task := newTrackedProgressTask(pg.startingGroup.TaskPending("TiKV"))
	pg.startingTasks = map[string]progressTask{"tikv": task}

	task.Start()

	doneCh := make(chan struct{})
	go func() {
		<-doneCh
		task.Done()
	}()
	close(doneCh)

	pg.waitBootStartingTasksSettled(time.Second)
	pg.closeStartingGroup()
	ui.PrintLines([]string{""})

	require.NoError(t, ui.Close())
	require.NoError(t, out.Close())

	lines := bytes.Split(bytes.TrimSpace(eventBuf.Bytes()), []byte("\n"))
	require.NotEmpty(t, lines, "expected event log to contain at least one line")

	doneIdx := -1
	blankIdx := -1
	for i, line := range lines {
		e, err := progressv2.DecodeEvent(line)
		require.NoError(t, err)

		if e.Type == progressv2.EventTaskState && e.Status != nil && *e.Status == progressv2.TaskStatusDone {
			doneIdx = i
		}
		if e.Type == progressv2.EventPrintLines && len(e.Lines) == 1 && e.Lines[0] == "" {
			blankIdx = i
		}
	}

	require.NotEqual(t, -1, doneIdx, "expected to see a task done event in event log")
	require.NotEqual(t, -1, blankIdx, "expected to see a blank PrintLines event in event log")
	require.Less(t, doneIdx, blankIdx, "expected task done to be logged before PrintLines output")
}
