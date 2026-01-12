package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

func TestBootExecutor_PreRun_GeneratesTiProxyCerts(t *testing.T) {
	dir := t.TempDir()

	plan := BootPlan{
		DataDir: dir,
		Shared:  proc.SharedOptions{Mode: proc.ModeNormal},
		Services: []ServicePlan{
			{ServiceID: proc.ServiceTiProxy.String()},
		},
	}

	executor := newBootExecutor(nil, nil)
	if err := executor.PreRun(context.Background(), plan); err != nil {
		t.Fatalf("PreRun: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "tiproxy.crt")); err != nil {
		t.Fatalf("missing tiproxy.crt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tiproxy.key")); err != nil {
		t.Fatalf("missing tiproxy.key: %v", err)
	}
}

func TestBootExecutor_PreRun_SkipsTiProxyCertsWhenDisabled(t *testing.T) {
	dir := t.TempDir()

	plan := BootPlan{
		DataDir:  dir,
		Shared:   proc.SharedOptions{Mode: proc.ModeNormal},
		Services: []ServicePlan{{ServiceID: proc.ServiceTiDB.String()}},
	}

	executor := newBootExecutor(nil, nil)
	if err := executor.PreRun(context.Background(), plan); err != nil {
		t.Fatalf("PreRun: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "tiproxy.crt")); err == nil {
		t.Fatalf("unexpected tiproxy.crt generated")
	}
	if _, err := os.Stat(filepath.Join(dir, "tiproxy.key")); err == nil {
		t.Fatalf("unexpected tiproxy.key generated")
	}
}
