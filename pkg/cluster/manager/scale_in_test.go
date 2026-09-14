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

func TestHasStandaloneDashboard(t *testing.T) {
	require.True(t, hasStandaloneDashboard(&spec.Specification{
		DashboardServers: []*spec.DashboardSpec{{Host: "10.0.0.50", Port: 12333}},
	}))
	require.False(t, hasStandaloneDashboard(&spec.Specification{}))
	require.False(t, hasStandaloneDashboard(&spec.Specification{
		PDServers: []*spec.PDSpec{
			{Host: "10.0.0.1", ClientPort: 2379},
		},
		TiDBServers: []*spec.TiDBSpec{
			{Host: "10.0.0.10", Port: 4000},
		},
	}))
	require.False(t, hasStandaloneDashboard(&spec.Specification{
		PDServers:        []*spec.PDSpec{{Host: "10.0.0.1", ClientPort: 2379}},
		DashboardServers: []*spec.DashboardSpec{},
	}))
}
