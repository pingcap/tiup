// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package task

import (
	"context"
	"os"
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/stretchr/testify/require"
)

func TestUpdateMetaRecordsGrafanaEntryCleanup(t *testing.T) {
	// UpdateMeta records a cleanup for every Grafana it removes, next to the
	// ones recorded before. Settling them is left to the refresh that follows
	// (see spec.GrafanaEntryCleanup).
	recorded := []*spec.GrafanaEntryCleanup{{Host: "owed", Port: 3000}}
	for _, test := range []struct {
		name     string
		managed  []string // hosts of the managed Grafana instances
		recorded []*spec.GrafanaEntryCleanup
		deleted  []string
		kept     int // managed Grafana instances left after the removal
		want     []*spec.GrafanaEntryCleanup
	}{
		{name: "removing the last Grafana is recorded",
			managed: []string{"gone"}, deleted: []string{"gone:3000"},
			want: []*spec.GrafanaEntryCleanup{{Host: "gone", Port: 3000}}},
		{name: "a removal that leaves a Grafana managed is recorded as well",
			managed: []string{"gone", "kept"}, recorded: recorded, deleted: []string{"gone:3000"}, kept: 1,
			want: []*spec.GrafanaEntryCleanup{{Host: "owed", Port: 3000}, {Host: "gone", Port: 3000}}},
		{name: "unrelated removals keep the recorded cleanups",
			recorded: recorded, deleted: []string{"n2:4000"}, want: recorded},
		{name: "removals join the recorded cleanups",
			managed: []string{"gone"}, recorded: recorded, deleted: []string{"gone:3000"},
			want: []*spec.GrafanaEntryCleanup{{Host: "owed", Port: 3000}, {Host: "gone", Port: 3000}}},
		{name: "nothing is owed without a Grafana removal",
			deleted: []string{"n2:4000"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// UpdateMeta goes through the global profile, so every case needs
			// its own and none of them can run in parallel.
			t.Setenv("TIUP_COMPONENT_DATA_DIR", t.TempDir())
			require.NoError(t, spec.Initialize("cluster"))
			topology := &spec.Specification{
				TiDBServers:                 []*spec.TiDBSpec{{Host: "n1", Port: 4000}, {Host: "n2", Port: 4000}},
				PendingGrafanaEntryCleanups: test.recorded,
			}
			for _, host := range test.managed {
				topology.Grafanas = append(topology.Grafanas, &spec.GrafanaSpec{Host: host, Port: 3000})
			}
			updateMeta := &UpdateMeta{
				cluster: "test", metadata: &spec.ClusterMeta{User: "tidb", Topology: topology}, deletedNodeIDs: test.deleted,
			}
			require.NoError(t, updateMeta.Execute(context.Background()))

			saved, err := spec.ClusterMetadata("test")
			require.NoError(t, err)
			require.Equal(t, test.want, saved.Topology.PendingGrafanaEntryCleanups)
			require.Len(t, saved.Topology.Grafanas, test.kept)
			// The record reaches the disk only: the in-memory topology still
			// lists every instance and is what UpdateTopology is given next.
			require.Equal(t, test.recorded, topology.PendingGrafanaEntryCleanups)
			require.Len(t, topology.Grafanas, len(test.managed))
		})
	}
}

func TestSettleGrafanaEntryCleanups(t *testing.T) {
	// UpdateMeta goes through the global profile, so the test needs its own.
	t.Setenv("TIUP_COMPONENT_DATA_DIR", t.TempDir())
	require.NoError(t, spec.Initialize("cluster"))
	metadata := &spec.ClusterMeta{User: "tidb", Topology: &spec.Specification{
		TiDBServers: []*spec.TiDBSpec{{Host: "n1", Port: 4000}},
	}}
	require.NoError(t, spec.SaveClusterMeta("test", metadata))
	metaFile := spec.GetSpecManager().Path("test", "meta.yaml")
	before, err := os.ReadFile(metaFile)
	require.NoError(t, err)

	updateTopology := &UpdateTopology{cluster: "test", metadata: metadata}
	// Nothing owed: the saved metadata is left as it is.
	require.NoError(t, updateTopology.settleGrafanaEntryCleanups())
	after, err := os.ReadFile(metaFile)
	require.NoError(t, err)
	require.Equal(t, before, after)

	// During a scale-in the in-memory topology predates the record
	// UpdateMeta has just saved, so it says nothing is owed while the saved
	// metadata does: the record on disk is the one to drop.
	recorded := []*spec.GrafanaEntryCleanup{{Host: "removed", Port: 3000}}
	saved := &spec.ClusterMeta{User: "tidb", Topology: &spec.Specification{
		TiDBServers:                 metadata.Topology.TiDBServers,
		PendingGrafanaEntryCleanups: recorded,
	}}
	require.NoError(t, spec.SaveClusterMeta("test", saved))
	require.NoError(t, updateTopology.settleGrafanaEntryCleanups())

	settled, err := spec.ClusterMetadata("test")
	require.NoError(t, err)
	require.Empty(t, settled.Topology.PendingGrafanaEntryCleanups)
	require.Len(t, settled.Topology.TiDBServers, 1)

	// A later refresh is given the saved topology, record included: it ends
	// without the record in memory as well.
	metadata.Topology.PendingGrafanaEntryCleanups = recorded
	require.NoError(t, spec.SaveClusterMeta("test", metadata))
	require.NoError(t, updateTopology.settleGrafanaEntryCleanups())
	require.Nil(t, metadata.Topology.PendingGrafanaEntryCleanups)
}

func TestSettleGrafanaEntryCleanupsReportsUnreadableMetadata(t *testing.T) {
	t.Setenv("TIUP_COMPONENT_DATA_DIR", t.TempDir())
	require.NoError(t, spec.Initialize("cluster"))
	// No cluster of that name exists: an unreadable metadata must surface
	// instead of silently leaving a record behind.
	err := (&UpdateTopology{
		cluster: "missing", metadata: &spec.ClusterMeta{Topology: &spec.Specification{}},
	}).settleGrafanaEntryCleanups()
	require.Error(t, err)
}
