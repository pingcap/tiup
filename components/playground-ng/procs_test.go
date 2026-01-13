package main

import (
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestAddPlannedProcInController_UsesPlanSharedOptions(t *testing.T) {
	runtimeDir := t.TempDir()
	plannedDir := t.TempDir()

	pg := NewPlayground(runtimeDir, 0)
	// Ensure planned dir is the single source of truth for proc creation in this
	// unit test. Real boot paths always have consistent runtime/planned data dir.
	pg.dataDir = ""
	pg.bootOptions = &BootOptions{ShOpt: proc.SharedOptions{Mode: proc.ModeNormal, PDMode: "pd", PortOffset: 1}}

	state := &controllerState{}

	shared := proc.SharedOptions{Mode: proc.ModeNextGen, PDMode: "pd", PortOffset: 42, ForcePull: true}
	plan := ServicePlan{
		ServiceID:   proc.ServiceTiDB.String(),
		ComponentID: proc.ComponentTiDB.String(),
		Shared: ServiceSharedPlan{
			Host:       "127.0.0.1",
			Port:       4000,
			StatusPort: 10080,
		},
		TiDB: &proc.TiDBPlan{},
	}

	inst, err := pg.addPlannedProcInController(state, plan, "/bin/tidb", utils.Version("v0.0.0"), shared, plannedDir)
	require.NoError(t, err)

	tdb, ok := inst.(*proc.TiDBInstance)
	require.True(t, ok, "unexpected instance type: %T", inst)
	require.Equal(t, shared, tdb.ShOpt)
	require.Equal(t, plannedDir, tdb.TiProxyCertDir)
	require.Equal(t, filepath.Join(plannedDir, "tidb-0"), tdb.Dir)
}

func TestAddPlannedProcInController_CreatesTiProxy(t *testing.T) {
	plannedDir := t.TempDir()

	pg := NewPlayground(t.TempDir(), 0)
	pg.dataDir = ""

	state := &controllerState{}

	plan := ServicePlan{
		ServiceID:   proc.ServiceTiProxy.String(),
		ComponentID: proc.ComponentTiProxy.String(),
		Shared: ServiceSharedPlan{
			Host:       "127.0.0.1",
			Port:       6000,
			StatusPort: 3080,
		},
		TiProxy: &proc.TiProxyPlan{PDAddrs: []string{"127.0.0.1:2379"}},
	}

	inst, err := pg.addPlannedProcInController(state, plan, "/bin/tiproxy", utils.Version("v0.0.0"), proc.SharedOptions{Mode: proc.ModeNormal, PDMode: "pd"}, plannedDir)
	require.NoError(t, err)

	tp, ok := inst.(*proc.TiProxyInstance)
	require.True(t, ok, "unexpected instance type: %T", inst)
	require.Equal(t, []string{"127.0.0.1:2379"}, tp.Plan.PDAddrs)
	require.Equal(t, filepath.Join(plannedDir, "tiproxy-0"), tp.Dir)
}

func TestPlaygroundVersionConstraintForService_NoImplicitDefault(t *testing.T) {
	pg := &Playground{bootOptions: &BootOptions{}}
	got := pg.versionConstraintForService(proc.ServiceTiProxy, "")
	require.Equal(t, "", got)
}
