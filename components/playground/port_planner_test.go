package main

import (
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/stretchr/testify/require"
)

func TestPortPlanner_PortConflictNone_WildcardConflictsWithSpecificHost(t *testing.T) {
	p := newPortPlanner(PortConflictNone)

	a, err := p.alloc("127.0.0.1", 10080, 0)
	require.NoError(t, err)

	b, err := p.alloc("0.0.0.0", 10080, 0)
	require.NoError(t, err)
	require.NotEqual(t, a, b)

	c, err := p.alloc("127.0.0.1", 10080, 0)
	require.NoError(t, err)
	require.NotEqual(t, a, c)
	require.NotEqual(t, b, c)
}

func TestBuildBootPlan_PortConflictNone_UniquePDPorts(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts, "--pd=2")

	plan := buildBootPlanForTest(t, opts, nil)

	require.Len(t, plan.Services, 5)
	require.Equal(t, "pd-0", plan.Services[0].Name)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, 2380, plan.Services[0].Shared.Port)
	require.Equal(t, 2379, plan.Services[0].Shared.StatusPort)

	require.Equal(t, "pd-1", plan.Services[1].Name)
	require.Equal(t, proc.ServicePD.String(), plan.Services[1].ServiceID)
	require.Equal(t, 2381, plan.Services[1].Shared.Port)
	require.Equal(t, 2382, plan.Services[1].Shared.StatusPort)
}
