package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestValidateBootOptionsPure_ModeNormal_AllowsEmptyCSEOptions(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	if err := ValidateBootOptionsPure(opts); err != nil {
		t.Fatalf("ValidateBootOptionsPure: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_RequiresBucket(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000",
				Bucket:     "",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("expected bucket error, got: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_RequiresEndpointHost(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("expected endpoint error, got: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_AllowsTrailingSlashInEndpoint(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000/",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	if err := ValidateBootOptionsPure(opts); err != nil {
		t.Fatalf("ValidateBootOptionsPure: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_RejectsEndpointWithPath(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000/minio",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path") {
		t.Fatalf("expected path error, got: %v", err)
	}
}

func TestValidateBootOptionsPure_PDModeMS_AllowsLatestAlias(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: utils.LatestVersionAlias,
	}

	if err := ValidateBootOptionsPure(opts); err != nil {
		t.Fatalf("ValidateBootOptionsPure: %v", err)
	}
}

func TestValidateBootOptionsPure_PDModeMS_RejectsOldVersion(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "v6.5.0",
	}

	err := ValidateBootOptionsPure(opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "microservices") {
		t.Fatalf("expected microservices error, got: %v", err)
	}
}

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

func TestPlanInstallByResolvedBinaryPath_TiKVWorker_MissingBinary(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "tikv-server")
	if err := os.WriteFile(base, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write base binary: %v", err)
	}

	dp := planInstallByResolvedBinaryPath(proc.ServiceTiKVWorker, "tikv", "v1.0.0", base, nil, false)
	if dp == nil || dp.DebugReason != "missing_binary" {
		t.Fatalf("expected missing_binary download plan, got: %+v", dp)
	}
	if want := filepath.Join(dir, "tikv-worker"); dp.DebugBinPath != want {
		t.Fatalf("unexpected debug bin path: %q", dp.DebugBinPath)
	}

	if err := os.WriteFile(filepath.Join(dir, "tikv-worker"), []byte("bin"), 0o755); err != nil {
		t.Fatalf("write required binary: %v", err)
	}
	if dp := planInstallByResolvedBinaryPath(proc.ServiceTiKVWorker, "tikv", "v1.0.0", base, nil, false); dp != nil {
		t.Fatalf("expected nil download plan when binary exists, got: %+v", dp)
	}
}

func TestPlanInstallByResolvedBinaryPath_NGMonitoring_MissingBinary(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "prometheus")
	if err := os.WriteFile(base, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write base binary: %v", err)
	}

	dp := planInstallByResolvedBinaryPath(proc.ServiceNGMonitoring, "prometheus", "v1.0.0", base, nil, false)
	if dp == nil || dp.DebugReason != "missing_binary" {
		t.Fatalf("expected missing_binary download plan, got: %+v", dp)
	}
	if want := filepath.Join(dir, "ng-monitoring-server"); dp.DebugBinPath != want {
		t.Fatalf("unexpected debug bin path: %q", dp.DebugBinPath)
	}

	if err := os.WriteFile(filepath.Join(dir, "ng-monitoring-server"), []byte("bin"), 0o755); err != nil {
		t.Fatalf("write required binary: %v", err)
	}
	if dp := planInstallByResolvedBinaryPath(proc.ServiceNGMonitoring, "prometheus", "v1.0.0", base, nil, false); dp != nil {
		t.Fatalf("expected nil download plan when binary exists, got: %+v", dp)
	}
}

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
	require.Equal(t, want, got)
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
