// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/stretchr/testify/require"
)

func TestIsScaledInPD(t *testing.T) {
	topo := &spec.Specification{
		PDServers: []*spec.PDSpec{
			{Host: "10.0.0.1", ClientPort: 2379},
			{Host: "10.0.0.2", ClientPort: 2379},
		},
		DashboardServers: []*spec.DashboardSpec{
			{Host: "10.0.0.50", Port: 12333},
		},
	}

	require.True(t, isScaledInPD(topo, []string{"10.0.0.1:2379"}))
	require.True(t, isScaledInPD(topo, []string{"10.0.0.2:2379", "10.0.0.50:12333"}))
	require.False(t, isScaledInPD(topo, []string{"10.0.0.50:12333"}))
	require.False(t, isScaledInPD(topo, nil))
}

func TestDashboardScaleInWarning(t *testing.T) {
	for _, tt := range []struct {
		name        string
		nodes       []string
		dashboard   bool
		wantWarning bool
	}{
		{"PD with standalone Dashboard", []string{"10.0.0.1:2379"}, true, true},
		{"non-PD with standalone Dashboard", []string{"10.0.0.10:4000"}, true, false},
		{"PD without standalone Dashboard", []string{"10.0.0.1:2379"}, false, false},
		{"non-PD without standalone Dashboard", []string{"10.0.0.10:4000"}, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := &spec.Specification{
				PDServers:   []*spec.PDSpec{{Host: "10.0.0.1", ClientPort: 2379}},
				TiDBServers: []*spec.TiDBSpec{{Host: "10.0.0.10", Port: 4000}},
			}
			// ScaleIn records PD removal before executing tasks, then checks
			// for Dashboard in the refreshed topology after successful scale-in.
			after := &spec.Specification{}
			if tt.dashboard {
				after.DashboardServers = []*spec.DashboardSpec{{Host: "10.0.0.50", Port: 12333}}
			}
			warning := dashboardScaleInWarning("test-cluster", isScaledInPD(before, tt.nodes), after)
			if tt.wantWarning {
				require.Contains(t, warning, "Since PD node(s) were scaled in")
				require.Contains(t, warning, tui.OsArgs0()+" restart test-cluster -R tidb-dashboard")
			} else {
				require.Empty(t, warning)
			}
		})
	}
}
