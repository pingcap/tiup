package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	"github.com/stretchr/testify/require"
)

type recordingExecutorSource struct {
	mu sync.Mutex

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
	s.mu.Lock()
	s.ensureInstalledCalls = append(s.ensureInstalledCalls, component+"@"+resolved)
	s.mu.Unlock()
	return nil
}

func (s *recordingExecutorSource) BinaryPath(component, resolved string) (string, error) {
	_ = resolved
	s.mu.Lock()
	s.binaryPathCalls = append(s.binaryPathCalls, component)
	s.mu.Unlock()
	if s.binaryPathByComponent != nil {
		if path := s.binaryPathByComponent[component]; path != "" {
			return path, nil
		}
	}
	return filepath.Join("/bin", component), nil
}

type blockingDownloadSource struct {
	ctx context.Context

	entered chan struct{}
	release chan struct{}

	mu      sync.Mutex
	current int
	max     int
}

func (s *blockingDownloadSource) ResolveVersion(_ string, constraint string) (string, error) {
	return constraint, nil
}

func (s *blockingDownloadSource) PlanInstall(proc.ServiceID, string, string, bool) (*DownloadPlan, error) {
	return nil, nil
}

func (s *blockingDownloadSource) EnsureInstalled(string, string) error {
	s.mu.Lock()
	s.current++
	if s.current > s.max {
		s.max = s.current
	}
	s.mu.Unlock()

	s.entered <- struct{}{}

	defer func() {
		s.mu.Lock()
		s.current--
		s.mu.Unlock()
	}()

	select {
	case <-s.release:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *blockingDownloadSource) BinaryPath(component, _ string) (string, error) {
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
	require.NoError(t, executor.PreRun(context.Background(), plan))

	_, err := os.Stat(filepath.Join(dir, "tiproxy.crt"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "tiproxy.key"))
	require.NoError(t, err)
}

func TestBootExecutor_PreRun_SkipsTiProxyCertsWhenDisabled(t *testing.T) {
	dir := t.TempDir()

	plan := BootPlan{
		DataDir:  dir,
		Shared:   proc.SharedOptions{Mode: proc.ModeNormal},
		Services: []ServicePlan{{ServiceID: proc.ServiceTiDB.String()}},
	}

	executor := newBootExecutor(nil, nil)
	require.NoError(t, executor.PreRun(context.Background(), plan))

	_, err := os.Stat(filepath.Join(dir, "tiproxy.crt"))
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(filepath.Join(dir, "tiproxy.key"))
	require.ErrorIs(t, err, os.ErrNotExist)
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

	require.NoError(t, executor.Download(context.Background(), plan))
	require.ElementsMatch(t, []string{"tidb@v1.0.0", "tikv@v1.0.0"}, src.ensureInstalledCalls)
}

func TestBootExecutor_AddProcs_CachesBinaryPathByComponentVersion(t *testing.T) {
	dir := t.TempDir()
	tidbBin := filepath.Join(dir, "tidb-server")
	require.NoError(t, os.WriteFile(tidbBin, []byte("#!/bin/sh\nexit 0\n"), 0o755))

	pg := NewPlayground(dir, 0)
	pg.startController()
	defer func() {
		if pg.controllerCancel != nil {
			pg.controllerCancel()
		}
		select {
		case <-pg.controllerDoneCh:
		case <-time.After(2 * time.Second):
			require.FailNow(t, "controller did not stop")
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

	require.NoError(t, executor.AddProcs(context.Background(), plan))

	procs := pg.Procs(proc.ServiceTiDB)
	require.Len(t, procs, 2)

	require.Len(t, src.binaryPathCalls, 1)
}

func TestBootExecutor_AddProcs_ResolvesRequiredBinaryPath(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	promBin := filepath.Join(binDir, "prometheus")
	ngBin := filepath.Join(binDir, "ng-monitoring-server")
	tikvServer := filepath.Join(binDir, "tikv-server")
	tikvWorker := filepath.Join(binDir, "tikv-worker")
	for _, path := range []string{promBin, ngBin, tikvServer, tikvWorker} {
		require.NoErrorf(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755), "write bin %s", path)
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
			require.FailNow(t, "controller did not stop")
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

	require.NoError(t, executor.AddProcs(context.Background(), plan))

	ngProcs := pg.Procs(proc.ServiceNGMonitoring)
	require.Len(t, ngProcs, 1)
	require.Equal(t, ngBin, ngProcs[0].Info().BinPath)

	workerProcs := pg.Procs(proc.ServiceTiKVWorker)
	require.Len(t, workerProcs, 1)
	require.Equal(t, tikvWorker, workerProcs[0].Info().BinPath)
}

func TestBootExecutor_Download_RespectsConcurrencyLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	total := maxParallelComponentDownloads * 3
	entered := make(chan struct{}, total)
	release := make(chan struct{})

	src := &blockingDownloadSource{
		ctx:     ctx,
		entered: entered,
		release: release,
	}
	executor := newBootExecutor(nil, src)

	plan := BootPlan{Downloads: make([]DownloadPlan, 0, total)}
	for i := 0; i < total; i++ {
		plan.Downloads = append(plan.Downloads, DownloadPlan{
			ComponentID:     "comp-" + strconv.Itoa(i),
			ResolvedVersion: "v1.0.0",
		})
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- executor.Download(ctx, plan)
	}()

	for i := 0; i < maxParallelComponentDownloads; i++ {
		select {
		case <-entered:
		case <-ctx.Done():
			require.FailNow(t, "downloads did not reach concurrency limit")
		}
	}

	src.mu.Lock()
	maxSeen := src.max
	current := src.current
	src.mu.Unlock()

	require.Equal(t, maxParallelComponentDownloads, current)
	require.LessOrEqual(t, maxSeen, maxParallelComponentDownloads)

	close(release)
	require.NoError(t, <-errCh)
}

func TestBootExecutor_ExecuteBootPlan_DownloadPreRunAddProcsStart(t *testing.T) {
	oldStdout := tuiv2output.Stdout.Get()
	tuiv2output.Stdout.Set(io.Discard)
	defer tuiv2output.Stdout.Set(oldStdout)

	dir := t.TempDir()
	writeFakeBin := func(name string) string {
		path := filepath.Join(dir, name)
		data := []byte("#!/bin/sh\nset -eu\ntouch \"$0.started\"\n")
		require.NoErrorf(t, os.WriteFile(path, data, 0o755), "write fake bin %s", name)
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
			require.FailNow(t, "controller did not stop")
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

	require.NoError(t, executor.Download(context.Background(), plan))
	require.ElementsMatch(t, []string{"prometheus@v1.0.0", "tiproxy@v1.0.0"}, src.ensureInstalledCalls)

	require.NoError(t, executor.PreRun(context.Background(), plan))
	_, err := os.Stat(filepath.Join(dir, "tiproxy.crt"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "tiproxy.key"))
	require.NoError(t, err)

	require.NoError(t, executor.AddProcs(context.Background(), plan))
	require.Len(t, pg.Procs(proc.ServicePrometheus), 1)
	require.Len(t, pg.Procs(proc.ServiceTiProxy), 1)

	for _, marker := range []string{promBin + ".started", tiproxyBin + ".started"} {
		_, err := os.Stat(marker)
		require.ErrorIs(t, err, os.ErrNotExist, "unexpected started marker before start: %s", marker)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	starter := newBootStarter(ctx, pg, pg.procsSnapshot(), nil)
	_, err = starter.startPlanned(plannedServicesFromBootPlan(plan))
	require.NoError(t, err)
	for _, marker := range []string{promBin + ".started", tiproxyBin + ".started"} {
		require.Eventually(t, func() bool {
			_, err := os.Stat(marker)
			return err == nil
		}, 2*time.Second, 10*time.Millisecond, "started marker not created: %s", marker)
	}
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
