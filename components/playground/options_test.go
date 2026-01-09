package main

import (
	"strings"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

func TestBootOptionsService_AllocatesAndReturnsStablePointer(t *testing.T) {
	opts := &BootOptions{}

	cfg1 := opts.Service(proc.ServicePD)
	if cfg1 == nil {
		t.Fatalf("expected non-nil config")
	}
	cfg2 := opts.Service(proc.ServicePD)
	if cfg1 != cfg2 {
		t.Fatalf("expected Service to return stable pointer")
	}
	if got := opts.Service(""); got != nil {
		t.Fatalf("expected nil for empty service ID, got %+v", got)
	}
}

func TestBootOptionsServiceConfig_ReturnsCopy(t *testing.T) {
	opts := &BootOptions{}
	opts.Service(proc.ServicePD).Num = 3

	cfg, ok := opts.ServiceConfig(proc.ServicePD)
	if !ok {
		t.Fatalf("expected config to exist")
	}
	if cfg.Num != 3 {
		t.Fatalf("unexpected config num: %d", cfg.Num)
	}

	cfg.Num = 100
	if got := opts.Service(proc.ServicePD).Num; got != 3 {
		t.Fatalf("expected stored config to be unchanged, got %d", got)
	}
}

func TestBootOptionsSortedServiceIDs_DeterministicOrder(t *testing.T) {
	opts := &BootOptions{}
	opts.Service(proc.ServiceTiDB)
	opts.Service(proc.ServicePD)
	opts.Service(proc.ServiceTiKV)

	got := opts.SortedServiceIDs()
	if len(got) != 3 {
		t.Fatalf("unexpected length: %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("expected sorted order, got %v", got)
		}
	}
}

func TestValidateServiceCountLimits_EnforcedMaxNum(t *testing.T) {
	opts := &BootOptions{
		Services: map[proc.ServiceID]*proc.Config{
			proc.ServiceTiKVWorker: {Num: 2},
		},
	}
	err := validateServiceCountLimits(opts)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "at most 1") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "TiKV Worker") {
		t.Fatalf("unexpected error: %v", err)
	}
}
