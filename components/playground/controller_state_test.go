package main

import (
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
