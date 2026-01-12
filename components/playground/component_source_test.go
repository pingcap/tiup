package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

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
