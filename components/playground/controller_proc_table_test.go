package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

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
	if rec == nil {
		t.Fatalf("missing proc record for pid=123")
	}
	name := info.Name()
	if rec.name != name {
		t.Fatalf("unexpected record name: %q", rec.name)
	}
	if state.procByName[name] != rec {
		t.Fatalf("procByName index not pointing to inserted record")
	}
	if rec.logFile != "/tmp/tidb.log" {
		t.Fatalf("unexpected record log file: %q", rec.logFile)
	}
	if rec.startedAt.IsZero() {
		t.Fatalf("expected startedAt to be set")
	}
	startedAt := rec.startedAt

	info2 := &proc.ProcessInfo{
		Service: proc.ServiceTiDBSystem,
		ID:      1,
		Proc:    osProc,
	}
	inst2 := &stubProcess{info: info2}
	state.upsertProcRecord(inst2)

	rec2 := state.procByPID[123]
	if rec2 != rec {
		t.Fatalf("expected record to be updated in-place")
	}
	name2 := info2.Name()
	if rec2.name != name2 {
		t.Fatalf("unexpected updated record name: %q", rec2.name)
	}
	if state.procByName[name2] != rec2 {
		t.Fatalf("procByName index not pointing to updated record")
	}
	if _, ok := state.procByName[name]; ok {
		t.Fatalf("expected old name index to be removed")
	}
	if rec2.logFile != "/tmp/tidb.log" {
		t.Fatalf("expected log file to be preserved, got %q", rec2.logFile)
	}
	if !rec2.startedAt.Equal(startedAt) {
		t.Fatalf("expected startedAt to be preserved")
	}
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
	if !ok || removed != inst {
		t.Fatalf("expected instance to be removed")
	}
	if got := state.procs[serviceID]; len(got) != 0 {
		t.Fatalf("expected procs list to be empty, got %d", len(got))
	}
	if rec := state.procByPID[456]; rec == nil || !rec.removedFromProcs {
		t.Fatalf("expected removedFromProcs=true, got %+v", rec)
	}

	snap := state.snapshotProcRecords()
	if len(snap) != 1 || snap[0].PID != 456 || !snap[0].Removed {
		t.Fatalf("unexpected proc record snapshot: %+v", snap)
	}
	if _, ok := state.removeProcByPID(serviceID, 456); ok {
		t.Fatalf("expected second removal attempt to fail")
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
			t.Fatalf("expected procByPID entry to be deleted")
		}
		if _, ok := state.procByName[name]; ok {
			t.Fatalf("expected procByName entry to be deleted")
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
			t.Fatalf("expected procByName entry to be deleted")
		}
		if _, ok := state.procByPID[790]; ok {
			t.Fatalf("expected procByPID entry to be deleted")
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

	if ok := state.removeProc(serviceID, inst); !ok {
		t.Fatalf("expected instance to be removed")
	}
	if rec := state.procByPID[555]; rec == nil || !rec.removedFromProcs {
		t.Fatalf("expected removedFromProcs=true, got %+v", rec)
	}
}
