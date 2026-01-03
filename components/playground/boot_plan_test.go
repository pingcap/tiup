package main

import (
	"io"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/spf13/pflag"
)

func newTestFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func TestTiFlashDefaultNum_DisabledOnOldVersion(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v6.5.0",
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}

	if got := opts.Service(proc.ServiceTiFlash).Num; got != 0 {
		t.Fatalf("unexpected tiflash default num on %s: %d", opts.Version, got)
	}

	plan, err := buildBootPlan(opts)
	if err != nil {
		t.Fatalf("buildBootPlan: %v", err)
	}
	if cfg := plan.BaseConfigs[proc.ServiceTiFlash]; cfg.Num != 0 {
		t.Fatalf("unexpected planned tiflash num: %d", cfg.Num)
	}
}

func TestTiFlashPlanWhen_RequiresTiDB(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
		},
		Version: "nightly",
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}

	// Simulate "--db 0".
	opts.Service(proc.ServiceTiDB).Num = 0

	plan, err := buildBootPlan(opts)
	if err != nil {
		t.Fatalf("buildBootPlan: %v", err)
	}
	if _, ok := plan.BaseConfigs[proc.ServiceTiFlashWrite]; ok {
		t.Fatalf("unexpected planned tiflash-write when TiDB is disabled")
	}
	if _, ok := plan.BaseConfigs[proc.ServiceTiFlashCompute]; ok {
		t.Fatalf("unexpected planned tiflash-compute when TiDB is disabled")
	}
}

func TestBuildBootPlan_RequiredServices(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}

	plan, err := buildBootPlan(opts)
	if err != nil {
		t.Fatalf("buildBootPlan: %v", err)
	}
	if plan.RequiredServices[proc.ServicePD] <= 0 {
		t.Fatalf("expected %s to be required", proc.ServicePD)
	}
	if plan.RequiredServices[proc.ServiceTiKV] <= 0 {
		t.Fatalf("expected %s to be required", proc.ServiceTiKV)
	}
	if plan.RequiredServices[proc.ServiceTiDB] <= 0 {
		t.Fatalf("expected %s to be required", proc.ServiceTiDB)
	}
}
