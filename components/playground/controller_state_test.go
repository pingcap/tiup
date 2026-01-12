package main

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

func TestControllerStateAllocID_PerServiceIndependent(t *testing.T) {
	state := &controllerState{}

	if got := state.allocID(proc.ServiceTiDB); got != 0 {
		t.Fatalf("unexpected id: %d", got)
	}
	if got := state.allocID(proc.ServiceTiDB); got != 1 {
		t.Fatalf("unexpected id: %d", got)
	}
	if got := state.allocID(proc.ServiceTiKV); got != 0 {
		t.Fatalf("unexpected id: %d", got)
	}
}

func TestControllerStateAppendProc_PanicsOnServiceMismatch(t *testing.T) {
	state := &controllerState{}
	inst := &stubProcess{info: &proc.ProcessInfo{Service: proc.ServiceTiDB}}

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	state.appendProc(proc.ServiceTiKV, inst)
}

func TestControllerStateWalkProcs_SortsByServiceID(t *testing.T) {
	state := &controllerState{
		procs: map[proc.ServiceID][]proc.Process{
			"b-service": {&stubProcess{info: &proc.ProcessInfo{Service: "b-service", ID: 0}}},
			"a-service": {&stubProcess{info: &proc.ProcessInfo{Service: "a-service", ID: 0}}},
		},
	}

	var got []proc.ServiceID
	if err := state.walkProcs(func(serviceID proc.ServiceID, inst proc.Process) error {
		got = append(got, serviceID)
		return nil
	}); err != nil {
		t.Fatalf("walkProcs: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("unexpected callback count: %d", len(got))
	}
	if got[0] != "a-service" || got[1] != "b-service" {
		t.Fatalf("unexpected order: %v", got)
	}
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
