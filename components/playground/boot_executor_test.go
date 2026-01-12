package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
)

type recordingExecutorSource struct {
	ensureInstalledCalls []string
	binaryPathCalls      []string

	binaryPathByComponent map[string]string
}

func (s *recordingExecutorSource) ResolveVersion(_ string, constraint string) (string, error) {
	return constraint, nil
}

func (s *recordingExecutorSource) PlanInstall(proc.ServiceID, string, string, bool) (*DownloadPlan, error) {
	return nil, nil
}

func (s *recordingExecutorSource) EnsureInstalled(component, resolved string) error {
	s.ensureInstalledCalls = append(s.ensureInstalledCalls, component+"@"+resolved)
	return nil
}

func (s *recordingExecutorSource) BinaryPath(component, resolved string) (string, error) {
	_ = resolved
	s.binaryPathCalls = append(s.binaryPathCalls, component)
	if s.binaryPathByComponent != nil {
		if path := s.binaryPathByComponent[component]; path != "" {
			return path, nil
		}
	}
	return filepath.Join("/bin", component), nil
}

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

func TestBootExecutor_Download_EnsuresInstalled(t *testing.T) {
	src := &recordingExecutorSource{}
	executor := newBootExecutor(nil, src)

	plan := BootPlan{
		Downloads: []DownloadPlan{
			{ComponentID: "tidb", ResolvedVersion: "v1.0.0"},
			{ComponentID: "tikv", ResolvedVersion: "v1.0.0"},
			// Invalid entry should be ignored.
			{ComponentID: "", ResolvedVersion: "v1.0.0"},
		},
	}

	if err := executor.Download(plan); err != nil {
		t.Fatalf("Download: %v", err)
	}

	if got := len(src.ensureInstalledCalls); got != 2 {
		t.Fatalf("unexpected ensureInstalled calls: %v", src.ensureInstalledCalls)
	}
	if src.ensureInstalledCalls[0] != "tidb@v1.0.0" {
		t.Fatalf("unexpected ensureInstalled[0]: %q", src.ensureInstalledCalls[0])
	}
	if src.ensureInstalledCalls[1] != "tikv@v1.0.0" {
		t.Fatalf("unexpected ensureInstalled[1]: %q", src.ensureInstalledCalls[1])
	}
}

func TestBootExecutor_AddProcs_CachesBinaryPathByComponentVersion(t *testing.T) {
	dir := t.TempDir()
	pg := NewPlayground(dir, 0)
	pg.startController()
	defer func() {
		if pg.controllerCancel != nil {
			pg.controllerCancel()
		}
		select {
		case <-pg.controllerDoneCh:
		case <-time.After(2 * time.Second):
			t.Fatalf("controller did not stop")
		}
	}()

	src := &recordingExecutorSource{
		binaryPathByComponent: map[string]string{
			proc.ComponentTiDB.String(): "/bin/tidb-server",
		},
	}
	executor := newBootExecutor(pg, src)

	plan := BootPlan{
		DataDir: dir,
		Shared:  proc.SharedOptions{Mode: proc.ModeNormal, PDMode: "pd"},
		Services: []ServicePlan{
			{
				Name:            "tidb-0",
				ServiceID:       proc.ServiceTiDB.String(),
				ComponentID:     proc.ComponentTiDB.String(),
				ResolvedVersion: "v1.0.0",
				Shared: ServiceSharedPlan{
					Dir:        filepath.Join(dir, "tidb-0"),
					Host:       "127.0.0.1",
					Port:       4000,
					StatusPort: 10080,
				},
				TiDB: &proc.TiDBPlan{},
			},
			{
				Name:            "tidb-1",
				ServiceID:       proc.ServiceTiDB.String(),
				ComponentID:     proc.ComponentTiDB.String(),
				ResolvedVersion: "v1.0.0",
				Shared: ServiceSharedPlan{
					Dir:        filepath.Join(dir, "tidb-1"),
					Host:       "127.0.0.1",
					Port:       4001,
					StatusPort: 10081,
				},
				TiDB: &proc.TiDBPlan{},
			},
		},
	}

	if err := executor.AddProcs(context.Background(), plan); err != nil {
		t.Fatalf("AddProcs: %v", err)
	}

	procs := pg.Procs(proc.ServiceTiDB)
	if got := len(procs); got != 2 {
		t.Fatalf("unexpected tidb procs: %d", got)
	}

	if got := len(src.binaryPathCalls); got != 1 {
		t.Fatalf("expected 1 BinaryPath call, got %d: %v", got, src.binaryPathCalls)
	}
}
