package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	"github.com/stretchr/testify/require"
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
	tidbBin := filepath.Join(dir, "tidb-server")
	if err := os.WriteFile(tidbBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write tidb bin: %v", err)
	}

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
			proc.ComponentTiDB.String(): tidbBin,
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

func TestBootExecutor_AddProcs_ResolvesRequiredBinaryPath(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}

	promBin := filepath.Join(binDir, "prometheus")
	ngBin := filepath.Join(binDir, "ng-monitoring-server")
	tikvServer := filepath.Join(binDir, "tikv-server")
	tikvWorker := filepath.Join(binDir, "tikv-worker")
	for _, path := range []string{promBin, ngBin, tikvServer, tikvWorker} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write bin %s: %v", path, err)
		}
	}

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
			proc.ComponentPrometheus.String(): promBin,
			proc.ComponentTiKV.String():       tikvServer,
		},
	}
	executor := newBootExecutor(pg, src)

	plan := BootPlan{
		DataDir: dir,
		Shared:  proc.SharedOptions{Mode: proc.ModeCSE, PDMode: "pd"},
		Services: []ServicePlan{
			{
				ServiceID:       proc.ServiceNGMonitoring.String(),
				ComponentID:     proc.ComponentPrometheus.String(),
				ResolvedVersion: "v1.0.0",
				Shared: proc.ServiceSharedPlan{
					Host: "127.0.0.1",
					Port: 12020,
				},
				NGMonitoring: &proc.NGMonitoringPlan{PDAddrs: []string{"127.0.0.1:2379"}},
			},
			{
				ServiceID:       proc.ServiceTiKVWorker.String(),
				ComponentID:     proc.ComponentTiKV.String(),
				ResolvedVersion: "v1.0.0",
				Shared: proc.ServiceSharedPlan{
					Host: "127.0.0.1",
					Port: 19000,
				},
				TiKVWorker: &proc.TiKVWorkerPlan{PDAddrs: []string{"127.0.0.1:2379"}},
			},
		},
	}

	if err := executor.AddProcs(context.Background(), plan); err != nil {
		t.Fatalf("AddProcs: %v", err)
	}

	ngProcs := pg.Procs(proc.ServiceNGMonitoring)
	if got := len(ngProcs); got != 1 {
		t.Fatalf("unexpected ng-monitoring procs: %d", got)
	}
	if got := ngProcs[0].Info().BinPath; got != ngBin {
		t.Fatalf("unexpected ng-monitoring binpath: %q", got)
	}

	workerProcs := pg.Procs(proc.ServiceTiKVWorker)
	if got := len(workerProcs); got != 1 {
		t.Fatalf("unexpected tikv-worker procs: %d", got)
	}
	if got := workerProcs[0].Info().BinPath; got != tikvWorker {
		t.Fatalf("unexpected tikv-worker binpath: %q", got)
	}
}

func TestBootExecutor_ExecuteBootPlan_DownloadPreRunAddProcsStart(t *testing.T) {
	oldStdout := tuiv2output.Stdout.Get()
	tuiv2output.Stdout.Set(io.Discard)
	defer tuiv2output.Stdout.Set(oldStdout)

	dir := t.TempDir()
	writeFakeBin := func(name string) string {
		path := filepath.Join(dir, name)
		data := []byte("#!/bin/sh\nset -eu\ntouch \"$0.started\"\n")
		if err := os.WriteFile(path, data, 0o755); err != nil {
			t.Fatalf("write fake bin %s: %v", name, err)
		}
		return path
	}

	promBin := writeFakeBin("prometheus-bin")
	tiproxyBin := writeFakeBin("tiproxy-bin")

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
			proc.ComponentPrometheus.String(): promBin,
			proc.ComponentTiProxy.String():    tiproxyBin,
		},
	}
	executor := newBootExecutor(pg, src)

	plan := BootPlan{
		DataDir: dir,
		Shared:  proc.SharedOptions{Mode: proc.ModeNormal, PDMode: "pd"},
		Downloads: []DownloadPlan{
			{ComponentID: proc.ComponentPrometheus.String(), ResolvedVersion: "v1.0.0"},
			{ComponentID: proc.ComponentTiProxy.String(), ResolvedVersion: "v1.0.0"},
		},
		Services: []ServicePlan{
			{
				ServiceID:       proc.ServicePrometheus.String(),
				ComponentID:     proc.ComponentPrometheus.String(),
				ResolvedVersion: "v1.0.0",
				Shared: proc.ServiceSharedPlan{
					Dir:  filepath.Join(dir, "prometheus-0"),
					Host: "127.0.0.1",
					Port: 9090,
				},
			},
			{
				ServiceID:       proc.ServiceTiProxy.String(),
				ComponentID:     proc.ComponentTiProxy.String(),
				ResolvedVersion: "v1.0.0",
				Shared: proc.ServiceSharedPlan{
					Dir:        filepath.Join(dir, "tiproxy-0"),
					Host:       "127.0.0.1",
					Port:       6000,
					StatusPort: 3080,
					UpTimeout:  1,
				},
				TiProxy: &proc.TiProxyPlan{PDAddrs: []string{"127.0.0.1:2379"}},
			},
		},
	}

	if err := executor.Download(plan); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if got := src.ensureInstalledCalls; len(got) != 2 || got[0] != "prometheus@v1.0.0" || got[1] != "tiproxy@v1.0.0" {
		t.Fatalf("unexpected ensureInstalled calls: %v", got)
	}

	if err := executor.PreRun(context.Background(), plan); err != nil {
		t.Fatalf("PreRun: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tiproxy.crt")); err != nil {
		t.Fatalf("missing tiproxy.crt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tiproxy.key")); err != nil {
		t.Fatalf("missing tiproxy.key: %v", err)
	}

	if err := executor.AddProcs(context.Background(), plan); err != nil {
		t.Fatalf("AddProcs: %v", err)
	}
	if got := len(pg.Procs(proc.ServicePrometheus)); got != 1 {
		t.Fatalf("unexpected prometheus procs: %d", got)
	}
	if got := len(pg.Procs(proc.ServiceTiProxy)); got != 1 {
		t.Fatalf("unexpected tiproxy procs: %d", got)
	}

	for _, marker := range []string{promBin + ".started", tiproxyBin + ".started"} {
		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("unexpected started marker before start: %s", marker)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	starter := newBootStarter(ctx, pg, pg.procsSnapshot(), nil)
	if _, err := starter.startPlanned(plannedServicesFromBootPlan(plan)); err != nil {
		t.Fatalf("startPlanned: %v", err)
	}

	waitMarker := func(path string) {
		deadline := time.Now().Add(2 * time.Second)
		for {
			if _, err := os.Stat(path); err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("started marker not created: %s", path)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitMarker(promBin + ".started")
	waitMarker(tiproxyBin + ".started")
}

func TestBootExecutor_PreRun_HandlersRunOnceInSortedOrder(t *testing.T) {
	oldHandlers := servicePreRunHandlers
	t.Cleanup(func() { servicePreRunHandlers = oldHandlers })

	var got []string
	servicePreRunHandlers = map[proc.ServiceID]preRunHandler{
		"b": func(_ context.Context, _ BootPlan, services []ServicePlan) error {
			got = append(got, "b:"+strconv.Itoa(len(services)))
			return nil
		},
		"a": func(_ context.Context, _ BootPlan, services []ServicePlan) error {
			got = append(got, "a:"+strconv.Itoa(len(services)))
			return nil
		},
	}

	executor := newBootExecutor(nil, nil)
	plan := BootPlan{
		Shared: proc.SharedOptions{Mode: proc.ModeNormal},
		Services: []ServicePlan{
			{ServiceID: "b"},
			{ServiceID: "a"},
			{ServiceID: "a"},
		},
	}
	require.NoError(t, executor.PreRun(context.Background(), plan))
	require.Equal(t, []string{"a:2", "b:1"}, got)
}
