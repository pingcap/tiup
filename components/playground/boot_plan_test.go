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

func TestPDAPIDefaults_InheritFromPDInMicroservicesMode(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "nightly",
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := fs.Parse([]string{
		"--pd=2",
		"--pd.host=1.2.3.4",
		"--pd.port=1234",
		"--pd.config=/tmp/pd.toml",
		"--pd.binpath=/tmp/pd",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}

	pdAPI := opts.Service(proc.ServicePDAPI)
	if pdAPI == nil {
		t.Fatalf("pd-api config is nil")
	}
	if pdAPI.Num != 2 {
		t.Fatalf("unexpected pd-api num: %d", pdAPI.Num)
	}
	if pdAPI.Host != "1.2.3.4" {
		t.Fatalf("unexpected pd-api host: %q", pdAPI.Host)
	}
	if pdAPI.Port != 1234 {
		t.Fatalf("unexpected pd-api port: %d", pdAPI.Port)
	}
	if pdAPI.ConfigPath != "/tmp/pd.toml" {
		t.Fatalf("unexpected pd-api config: %q", pdAPI.ConfigPath)
	}
	if pdAPI.BinPath != "/tmp/pd" {
		t.Fatalf("unexpected pd-api binpath: %q", pdAPI.BinPath)
	}

	plan, err := buildBootPlan(opts)
	if err != nil {
		t.Fatalf("buildBootPlan: %v", err)
	}
	if _, ok := plan.BaseConfigs[proc.ServicePD]; ok {
		t.Fatalf("unexpected planned pd in microservices mode")
	}
	if cfg := plan.BaseConfigs[proc.ServicePDAPI]; cfg.Num != 2 {
		t.Fatalf("unexpected planned pd-api num: %d", cfg.Num)
	}
	if plan.RequiredServices[proc.ServicePDAPI] <= 0 {
		t.Fatalf("expected %s to be required", proc.ServicePDAPI)
	}
}

func TestPDAPIDefaults_OverridePD(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "nightly",
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := fs.Parse([]string{
		"--pd.host=1.2.3.4",
		"--pd.port=1234",
		"--pd.api.host=5.6.7.8",
		"--pd.api.port=2345",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}

	pdAPI := opts.Service(proc.ServicePDAPI)
	if pdAPI == nil {
		t.Fatalf("pd-api config is nil")
	}
	if pdAPI.Host != "5.6.7.8" {
		t.Fatalf("unexpected pd-api host: %q", pdAPI.Host)
	}
	if pdAPI.Port != 2345 {
		t.Fatalf("unexpected pd-api port: %d", pdAPI.Port)
	}
}
