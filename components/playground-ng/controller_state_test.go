package main

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/stretchr/testify/require"
)

func TestControllerStateAllocID_PerServiceIndependent(t *testing.T) {
	state := &controllerState{}

	require.Equal(t, 0, state.allocID(proc.ServiceTiDB))
	require.Equal(t, 1, state.allocID(proc.ServiceTiDB))
	require.Equal(t, 0, state.allocID(proc.ServiceTiKV))
}

func TestControllerStateAppendProc_PanicsOnServiceMismatch(t *testing.T) {
	state := &controllerState{}
	inst := &stubProcess{info: &proc.ProcessInfo{Service: proc.ServiceTiDB}}

	require.Panics(t, func() {
		state.appendProc(proc.ServiceTiKV, inst)
	})
}

func TestControllerStateWalkProcs_SortsByServiceID(t *testing.T) {
	state := &controllerState{
		procs: map[proc.ServiceID][]proc.Process{
			"b-service": {&stubProcess{info: &proc.ProcessInfo{Service: "b-service", ID: 0}}},
			"a-service": {&stubProcess{info: &proc.ProcessInfo{Service: "a-service", ID: 0}}},
		},
	}

	var got []proc.ServiceID
	err := state.walkProcs(func(serviceID proc.ServiceID, inst proc.Process) error {
		got = append(got, serviceID)
		return nil
	})
	require.NoError(t, err)

	require.Len(t, got, 2)
	require.Equal(t, []proc.ServiceID{"a-service", "b-service"}, got)
}

func TestControllerStateUpsertProcRecord_InsertsAndUpdatesIndexes(t *testing.T) {
	cmd := &exec.Cmd{Process: &os.Process{Pid: 123}}
	osProc := &stubOSProcess{pid: 123, cmd: cmd}

	info := &proc.ProcessInfo{
		Service: proc.ServiceTiDB,
		ID:      0,
		Proc:    osProc,
	}
	inst := &stubProcess{info: info, logFile: "/tmp/tidb.log"}

	state := &controllerState{}
	state.upsertProcRecord(inst)

	rec := state.procByPID[123]
	require.NotNil(t, rec)
	name := info.Name()
	require.Equal(t, name, rec.name)
	require.Same(t, rec, state.procByName[name])
	require.Equal(t, "/tmp/tidb.log", rec.logFile)
	require.False(t, rec.startedAt.IsZero())
	startedAt := rec.startedAt

	info2 := &proc.ProcessInfo{
		Service: proc.ServiceTiDBSystem,
		ID:      1,
		Proc:    osProc,
	}
	inst2 := &stubProcess{info: info2}
	state.upsertProcRecord(inst2)

	rec2 := state.procByPID[123]
	require.Same(t, rec, rec2)
	name2 := info2.Name()
	require.Equal(t, name2, rec2.name)
	require.Same(t, rec2, state.procByName[name2])
	if _, ok := state.procByName[name]; ok {
		require.FailNow(t, "expected old name index to be removed")
	}
	require.Equal(t, "/tmp/tidb.log", rec2.logFile)
	require.True(t, rec2.startedAt.Equal(startedAt))
}

func TestControllerStateRemoveProcByPID_MarksProcRemoved(t *testing.T) {
	cmd := &exec.Cmd{Process: &os.Process{Pid: 456}}
	osProc := &stubOSProcess{pid: 456, cmd: cmd}

	serviceID := proc.ServiceTiKV
	info := &proc.ProcessInfo{
		Service: serviceID,
		ID:      0,
		Proc:    osProc,
	}
	inst := &stubProcess{info: info}

	state := &controllerState{
		procs: make(map[proc.ServiceID][]proc.Process),
	}
	state.appendProc(serviceID, inst)
	state.upsertProcRecord(inst)

	removed, ok := state.removeProcByPID(serviceID, 456)
	require.True(t, ok)
	require.Equal(t, inst, removed)
	require.Empty(t, state.procs[serviceID])
	require.True(t, state.procByPID[456].removedFromProcs)

	snap := state.snapshotProcRecords()
	require.Len(t, snap, 1)
	require.Equal(t, 456, snap[0].PID)
	require.True(t, snap[0].Removed)
	if _, ok := state.removeProcByPID(serviceID, 456); ok {
		require.FailNow(t, "expected second removal attempt to fail")
	}
}

func TestControllerStateDeleteProcRecord(t *testing.T) {
	t.Run("ByPID", func(t *testing.T) {
		cmd := &exec.Cmd{Process: &os.Process{Pid: 789}}
		osProc := &stubOSProcess{pid: 789, cmd: cmd}

		info := &proc.ProcessInfo{
			Service: proc.ServicePD,
			ID:      0,
			Proc:    osProc,
		}
		inst := &stubProcess{info: info}

		state := &controllerState{}
		state.upsertProcRecord(inst)

		name := info.Name()
		state.deleteProcRecord(789, "")

		if _, ok := state.procByPID[789]; ok {
			require.FailNow(t, "expected procByPID entry to be deleted")
		}
		if _, ok := state.procByName[name]; ok {
			require.FailNow(t, "expected procByName entry to be deleted")
		}
	})

	t.Run("ByName", func(t *testing.T) {
		cmd := &exec.Cmd{Process: &os.Process{Pid: 790}}
		osProc := &stubOSProcess{pid: 790, cmd: cmd}

		info := &proc.ProcessInfo{
			Service: proc.ServicePDAPI,
			ID:      0,
			Proc:    osProc,
		}
		inst := &stubProcess{info: info}

		state := &controllerState{}
		state.upsertProcRecord(inst)

		name := info.Name()
		state.deleteProcRecord(0, name)

		if _, ok := state.procByName[name]; ok {
			require.FailNow(t, "expected procByName entry to be deleted")
		}
		if _, ok := state.procByPID[790]; ok {
			require.FailNow(t, "expected procByPID entry to be deleted")
		}
	})
}

func TestControllerStateRemoveProc_MarksProcRemoved(t *testing.T) {
	cmd := &exec.Cmd{Process: &os.Process{Pid: 555}}
	osProc := &stubOSProcess{pid: 555, cmd: cmd}

	serviceID := proc.ServiceTiDB
	info := &proc.ProcessInfo{
		Service: serviceID,
		ID:      0,
		Proc:    osProc,
	}
	inst := &stubProcess{info: info}

	state := &controllerState{
		procs: make(map[proc.ServiceID][]proc.Process),
	}
	state.appendProc(serviceID, inst)
	state.upsertProcRecord(inst)

	require.True(t, state.removeProc(serviceID, inst))
	require.True(t, state.procByPID[555].removedFromProcs)
}

type stubOSProcess struct {
	pid    int
	cmd    *exec.Cmd
	uptime string
}

func (p *stubOSProcess) Start() error                     { return nil }
func (p *stubOSProcess) Wait() error                      { return nil }
func (p *stubOSProcess) Pid() int                         { return p.pid }
func (p *stubOSProcess) Uptime() string                   { return p.uptime }
func (p *stubOSProcess) SetOutputFile(fname string) error { return nil }
func (p *stubOSProcess) Cmd() *exec.Cmd                   { return p.cmd }

type stubProcess struct {
	info    *proc.ProcessInfo
	logFile string
}

func (p *stubProcess) Info() *proc.ProcessInfo           { return p.info }
func (p *stubProcess) Prepare(ctx context.Context) error { return nil }
func (p *stubProcess) LogFile() string                   { return p.logFile }
