package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	pgservice "github.com/pingcap/tiup/components/playground-ng/service"
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
		Host:    "127.0.0.1",
	}
	opts.Service(proc.ServicePD).Num = 1

	require.NoError(t, ValidateBootOptionsPure(opts))
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
		Host:    "127.0.0.1",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bucket")
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
		Host:    "127.0.0.1",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
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
		Host:    "127.0.0.1",
	}
	opts.Service(proc.ServicePD).Num = 1

	require.NoError(t, ValidateBootOptionsPure(opts))
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
		Host:    "127.0.0.1",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "path")
}

func TestValidateBootOptionsPure_PDModeMS_AllowsLatestAlias(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: utils.LatestVersionAlias,
		Host:    "127.0.0.1",
	}

	require.NoError(t, ValidateBootOptionsPure(opts))
}

func TestValidateBootOptionsPure_PDModeMS_RejectsOldVersion(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "v6.5.0",
		Host:    "127.0.0.1",
	}

	err := ValidateBootOptionsPure(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "microservices")
}

func TestBootOptionsService_AllocatesAndReturnsStablePointer(t *testing.T) {
	opts := &BootOptions{}

	cfg1 := opts.Service(proc.ServicePD)
	require.NotNil(t, cfg1)
	cfg2 := opts.Service(proc.ServicePD)
	require.Same(t, cfg1, cfg2)
	if got := opts.Service(""); got != nil {
		require.FailNowf(t, "expected nil for empty service ID", "got %+v", got)
	}
}

func TestBootOptionsServiceConfig_ReturnsCopy(t *testing.T) {
	opts := &BootOptions{}
	opts.Service(proc.ServicePD).Num = 3

	cfg, ok := opts.ServiceConfig(proc.ServicePD)
	require.True(t, ok)
	require.Equal(t, 3, cfg.Num)

	cfg.Num = 100
	require.Equal(t, 3, opts.Service(proc.ServicePD).Num)
}

func TestBootOptionsSortedServiceIDs_DeterministicOrder(t *testing.T) {
	opts := &BootOptions{}
	opts.Service(proc.ServiceTiDB)
	opts.Service(proc.ServicePD)
	opts.Service(proc.ServiceTiKV)

	got := opts.SortedServiceIDs()
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		require.False(t, got[i-1] > got[i], "expected sorted order, got %v", got)
	}
}

func TestValidateServiceCountLimits_EnforcedMaxNum(t *testing.T) {
	opts := &BootOptions{
		Services: map[proc.ServiceID]*proc.Config{
			proc.ServiceTiKVWorker: {Num: 2},
		},
	}
	err := validateServiceCountLimits(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "at most 1")
	require.Contains(t, err.Error(), "TiKV Worker")
}

func TestPlanInstallByResolvedBinaryPath_TiKVWorker_MissingBinary(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "tikv-server")
	require.NoError(t, os.WriteFile(base, []byte("bin"), 0o755))

	dp := planInstallByResolvedBinaryPath(proc.ServiceTiKVWorker, "tikv", "v1.0.0", base, nil, false)
	require.NotNil(t, dp)
	require.Equal(t, "missing_binary", dp.DebugReason)
	require.Equal(t, filepath.Join(dir, "tikv-worker"), dp.DebugBinPath)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "tikv-worker"), []byte("bin"), 0o755))
	if dp := planInstallByResolvedBinaryPath(proc.ServiceTiKVWorker, "tikv", "v1.0.0", base, nil, false); dp != nil {
		require.FailNowf(t, "expected nil download plan when binary exists", "got: %+v", dp)
	}
}

func TestPlanInstallByResolvedBinaryPath_NGMonitoring_MissingBinary(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "prometheus")
	require.NoError(t, os.WriteFile(base, []byte("bin"), 0o755))

	dp := planInstallByResolvedBinaryPath(proc.ServiceNGMonitoring, "prometheus", "v1.0.0", base, nil, false)
	require.NotNil(t, dp)
	require.Equal(t, "missing_binary", dp.DebugReason)
	require.Equal(t, filepath.Join(dir, "ng-monitoring-server"), dp.DebugBinPath)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ng-monitoring-server"), []byte("bin"), 0o755))
	if dp := planInstallByResolvedBinaryPath(proc.ServiceNGMonitoring, "prometheus", "v1.0.0", base, nil, false); dp != nil {
		require.FailNowf(t, "expected nil download plan when binary exists", "got: %+v", dp)
	}
}

func ensureTestServiceSpec(t *testing.T, spec pgservice.Spec) {
	t.Helper()
	if _, ok := pgservice.SpecFor(spec.ServiceID); ok {
		return
	}
	require.NoErrorf(t, pgservice.Register(spec), "register %s", spec.ServiceID)
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
	require.NoError(t, err)
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
	require.Error(t, err)
	require.Contains(t, err.Error(), "service dependency cycle detected")
}
