package main

import (
	"strings"
	"testing"

	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

func ensureTestServiceSpec(t *testing.T, spec pgservice.Spec) {
	t.Helper()
	if _, ok := pgservice.SpecFor(spec.ServiceID); ok {
		return
	}
	if err := pgservice.Register(spec); err != nil {
		t.Fatalf("register %s: %v", spec.ServiceID, err)
	}
}

func TestTopoSortServiceIDs_StableOrder(t *testing.T) {
	a := proc.ServiceID("test-toposort-a")
	b := proc.ServiceID("test-toposort-b")
	c := proc.ServiceID("test-toposort-c")

	ensureTestServiceSpec(t, pgservice.Spec{
		ServiceID: a,
		NewProc: func(pgservice.ControllerRuntime, pgservice.NewProcParams) (proc.Process, error) {
			return nil, nil
		},
	})
	ensureTestServiceSpec(t, pgservice.Spec{
		ServiceID: b,
		NewProc: func(pgservice.ControllerRuntime, pgservice.NewProcParams) (proc.Process, error) {
			return nil, nil
		},
	})
	ensureTestServiceSpec(t, pgservice.Spec{
		ServiceID:  c,
		StartAfter: []proc.ServiceID{a},
		NewProc: func(pgservice.ControllerRuntime, pgservice.NewProcParams) (proc.Process, error) {
			return nil, nil
		},
	})

	got, err := topoSortServiceIDs([]proc.ServiceID{c, b, a})
	if err != nil {
		t.Fatalf("topoSortServiceIDs: %v", err)
	}
	want := []proc.ServiceID{a, b, c}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected order: got %v want %v", got, want)
	}
}

func TestTopoSortServiceIDs_Cycle(t *testing.T) {
	x := proc.ServiceID("test-toposort-cycle-x")
	y := proc.ServiceID("test-toposort-cycle-y")

	ensureTestServiceSpec(t, pgservice.Spec{
		ServiceID:  x,
		StartAfter: []proc.ServiceID{y},
		NewProc: func(pgservice.ControllerRuntime, pgservice.NewProcParams) (proc.Process, error) {
			return nil, nil
		},
	})
	ensureTestServiceSpec(t, pgservice.Spec{
		ServiceID:  y,
		StartAfter: []proc.ServiceID{x},
		NewProc: func(pgservice.ControllerRuntime, pgservice.NewProcParams) (proc.Process, error) {
			return nil, nil
		},
	})

	_, err := topoSortServiceIDs([]proc.ServiceID{x, y})
	if err == nil {
		t.Fatalf("expected cycle error")
	}
	if !strings.Contains(err.Error(), "service dependency cycle detected") {
		t.Fatalf("unexpected error: %v", err)
	}
}
