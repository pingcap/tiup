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
	"encoding/json"
	"errors"
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// previousTiDBInstance returns the instance list an earlier call leaves in
// the output, which the function under test must pass through untouched.
func previousTiDBInstance() []spec.Instance {
	return (&spec.TiDBComponent{Topology: &spec.Specification{
		TiDBServers: []*spec.TiDBSpec{{Host: "tidb", Port: 4000}},
	}}).Instances()
}

// requirePassthrough checks what the two functions have in common: the
// operations and the instance list an earlier call left in the output come
// back untouched, followed by the surviving instances.
func requirePassthrough(t *testing.T, ops []clientv3.Op, previous, retained []spec.Instance, wantIDs []string) {
	t.Helper()
	require.NotEmpty(t, ops)
	require.True(t, ops[0].IsPut())
	require.Equal(t, "/previous", string(ops[0].KeyBytes()))
	require.Equal(t, "untouched", string(ops[0].ValueBytes()))
	require.Same(t, previous[0], retained[0])
	var ids []string
	for _, instance := range retained[1:] {
		ids = append(ids, instance.ID())
	}
	require.Equal(t, wantIDs, ids)
}

func TestUpdateInstancesAndOps(t *testing.T) {
	// Prometheus and Alertmanager keep the rule they always had: with no
	// surviving instance their discovery entry is deleted, so that a removal
	// whose PD update failed is converged by every later refresh.
	for _, component := range []string{spec.ComponentPrometheus, spec.ComponentAlertmanager} {
		for _, test := range []struct {
			name    string
			empty   bool
			deleted []string
			wantIDs []string
		}{
			{name: "empty", empty: true},
			{name: "empty with unrelated deletion", empty: true, deleted: []string{"other:1234"}},
			{name: "all retained", wantIDs: []string{"first:3000", "second:3000"}},
			{name: "first deleted", deleted: []string{"first:3000"}, wantIDs: []string{"second:3000"}},
			{name: "all deleted", deleted: []string{"first:3000", "second:3000"}},
			{name: "unrelated deletion", deleted: []string{"other:1234"}, wantIDs: []string{"first:3000", "second:3000"}},
		} {
			t.Run(component+"/"+test.name, func(t *testing.T) {
				topo := &spec.Specification{
					Monitors: []*spec.PrometheusSpec{
						{Host: "first", Port: 3000}, {Host: "second", Port: 3000},
					},
					Alertmanagers: []*spec.AlertmanagerSpec{
						{Host: "first", WebPort: 3000}, {Host: "second", WebPort: 3000},
					},
				}
				instances := spec.FindComponent(topo, component).Instances()
				if test.empty {
					instances = nil
				}
				previous := previousTiDBInstance()
				ops, retained := updateInstancesAndOps(
					[]clientv3.Op{clientv3.OpPut("/previous", "untouched")}, previous,
					set.NewStringSet(test.deleted...), instances, component,
				)
				requirePassthrough(t, ops, previous, retained, test.wantIDs)
				if len(test.wantIDs) == 0 {
					require.Len(t, ops, 2)
					require.True(t, ops[1].IsDelete())
					require.Equal(t, "/topology/"+component, string(ops[1].KeyBytes()))
				} else {
					require.Len(t, ops, 1)
				}
			})
		}
	}
}

func TestUpdateGrafanaAndOps(t *testing.T) {
	// The cases walk through the rule documented on updateGrafanaAndOps.
	// TiUP writes a Grafana discovery entry as the address and deploy path
	// of a managed instance; only the address matters here.
	entry := func(ip string, port int) string {
		data, err := json.Marshal(componentTopology{IP: ip, Port: port, DeployPath: "/deploy/grafana"})
		require.NoError(t, err)
		return string(data)
	}
	owed := []*spec.GrafanaEntryCleanup{{Host: "removed", Port: 3000}}
	for _, test := range []struct {
		name     string
		managed  []string                    // hosts of the Grafana instances in the topology
		deleted  []string                    // node IDs removed by this run
		recorded []*spec.GrafanaEntryCleanup // cleanups owed before this run
		entry    string                      // current value of the discovery entry, empty when absent
		read     bool                        // whether deciding needs the entry
		delete   bool
		wantIDs  []string
	}{
		// While a managed Grafana survives it takes over the entry, as before.
		{name: "surviving Grafana takes over the entry",
			managed: []string{"first", "second"}, wantIDs: []string{"first:3000", "second:3000"}},
		{name: "surviving Grafana after a removal",
			managed: []string{"first", "second"}, deleted: []string{"first:3000"}, wantIDs: []string{"second:3000"}},
		// With no managed Grafana and nothing owed, the entry is not TiUP's:
		// it may belong to a Grafana handed over with detach-grafana, or to one
		// an operator runs outside TiUP. It is left alone, and not even read.
		{name: "entry of a detached Grafana is left alone", entry: entry("gone", 3000)},
		{name: "entry of a Grafana outside TiUP is left alone", entry: entry("192.0.2.1", 3000)},
		{name: "missing entry stays missing"},
		// A scale-in of the last Grafana removes the instance the entry points
		// at, so the entry is deleted in the same run ...
		{name: "entry at the instance removed in this run is deleted",
			managed: []string{"gone"}, deleted: []string{"gone:3000"},
			entry: entry("gone", 3000), read: true, delete: true},
		// ... and only that entry: one pointing elsewhere, say at a Grafana
		// detached earlier, is not TiUP's to delete.
		{name: "entry elsewhere during a removal is left alone",
			managed: []string{"gone"}, deleted: []string{"gone:3000"},
			entry: entry("elsewhere", 3000), read: true},
		{name: "missing entry during a removal stays missing",
			managed: []string{"gone"}, deleted: []string{"gone:3000"}, read: true},
		// When the PD update of that scale-in fails, the saved metadata no
		// longer lists the instance. The recorded cleanup is what lets every
		// later refresh retry the deletion.
		{name: "recorded cleanup is retried by a later refresh",
			recorded: owed, entry: entry("removed", 3000), read: true, delete: true},
		{name: "recorded cleanup does not reach an entry at another address",
			recorded: owed, entry: entry("moved", 3000), read: true},
		{name: "recorded cleanup with the entry already gone", recorded: owed, read: true},
		{name: "recorded and removed instances both count",
			managed: []string{"gone"}, deleted: []string{"gone:3000"},
			recorded: owed, entry: entry("removed", 3000), read: true, delete: true},
		// Hosts are compared by address, so IPv6 spellings name one instance.
		{name: "IPv6 spelling differences name the same instance",
			recorded: []*spec.GrafanaEntryCleanup{{Host: "2001:db8::1", Port: 3000}},
			entry:    entry("2001:db8:0:0:0:0:0:1", 3000), read: true, delete: true},
		// A value TiUP did not write is not the leftover of an instance it
		// removed; decoding it must not fail the refresh either.
		{name: "undecodable entry is left alone", recorded: owed, entry: "not json", read: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			topo := &spec.Specification{}
			for _, host := range test.managed {
				topo.Grafanas = append(topo.Grafanas, &spec.GrafanaSpec{Host: host, Port: 3000})
			}
			previous := previousTiDBInstance()
			read := false
			ops, retained, cleanup, err := updateGrafanaAndOps(
				[]clientv3.Op{clientv3.OpPut("/previous", "untouched")}, previous,
				set.NewStringSet(test.deleted...), (&spec.GrafanaComponent{Topology: topo}).Instances(), test.recorded,
				func() (*topologyEntry, error) {
					read = true
					if test.entry == "" {
						return nil, nil
					}
					return &topologyEntry{value: []byte(test.entry), modRevision: 42}, nil
				},
			)
			require.NoError(t, err)
			require.Equal(t, test.read, read, "the entry is read exactly when a cleanup is owed")
			requirePassthrough(t, ops, previous, retained, test.wantIDs)
			// The decision never adds an operation. The entry to delete is
			// handed back as it was read, revision included, and Execute owns
			// the delete: only while the key still carries that revision.
			require.Len(t, ops, 1)
			if test.delete {
				require.Equal(t, &topologyEntry{value: []byte(test.entry), modRevision: 42}, cleanup)
			} else {
				require.Nil(t, cleanup)
			}
		})
	}
}

func TestUpdateGrafanaAndOpsPropagatesReadError(t *testing.T) {
	// The entry is read only when a cleanup is owed, and a failed read must
	// then not pass for a missing entry: the refresh fails and the next one
	// retries, with the cleanup still recorded.
	readFailed := errors.New("read failed")
	for _, test := range []struct {
		name     string
		managed  []string
		deleted  []string
		recorded []*spec.GrafanaEntryCleanup
	}{
		{name: "removed in this run", managed: []string{"gone"}, deleted: []string{"gone:3000"}},
		{name: "recorded earlier", recorded: []*spec.GrafanaEntryCleanup{{Host: "removed", Port: 3000}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			topo := &spec.Specification{}
			for _, host := range test.managed {
				topo.Grafanas = append(topo.Grafanas, &spec.GrafanaSpec{Host: host, Port: 3000})
			}
			_, _, _, err := updateGrafanaAndOps(
				nil, nil, set.NewStringSet(test.deleted...), (&spec.GrafanaComponent{Topology: topo}).Instances(), test.recorded,
				func() (*topologyEntry, error) { return nil, readFailed },
			)
			require.ErrorIs(t, err, readFailed)
		})
	}
}

func TestLastGrafanaScaleInDeletesEntryOfEarlierRemoval(t *testing.T) {
	// Two Grafana instances are managed and the entry points at the first. The
	// first is scaled in, but the refresh that should move the entry to the
	// second fails. By the time the second is scaled in, the entry still points
	// at an instance TiUP removed, and only the record of the first removal can
	// tell that leftover from an entry an operator maintains.
	t.Setenv("TIUP_COMPONENT_DATA_DIR", t.TempDir())
	require.NoError(t, spec.Initialize("cluster"))
	metadata := &spec.ClusterMeta{User: "tidb", Topology: &spec.Specification{
		TiDBServers: []*spec.TiDBSpec{{Host: "n1", Port: 4000}},
		Grafanas:    []*spec.GrafanaSpec{{Host: "first", Port: 3000}, {Host: "second", Port: 3000}},
	}}
	require.NoError(t, spec.SaveClusterMeta("test", metadata))
	value, err := json.Marshal(componentTopology{IP: "first", Port: 3000, DeployPath: "/deploy/grafana-3000"})
	require.NoError(t, err)
	entry := &topologyEntry{value: value, modRevision: 42}

	// The refresh of this scale-in fails, so nothing settles its record.
	require.NoError(t, (&UpdateMeta{
		cluster: "test", metadata: metadata, deletedNodeIDs: []string{"first:3000"},
	}).Execute(context.Background()))
	between, err := spec.ClusterMetadata("test")
	require.NoError(t, err)

	require.NoError(t, (&UpdateMeta{
		cluster: "test", metadata: between, deletedNodeIDs: []string{"second:3000"},
	}).Execute(context.Background()))
	// UpdateTopology is given the metadata as it was before UpdateMeta ran.
	_, _, cleanup, err := updateGrafanaAndOps(nil, nil, set.NewStringSet("second:3000"),
		(&spec.GrafanaComponent{Topology: between.Topology}).Instances(), between.Topology.PendingGrafanaEntryCleanups,
		func() (*topologyEntry, error) { return entry, nil })
	require.NoError(t, err)
	require.Same(t, entry, cleanup)
}

func TestUpdateTopologyOp(t *testing.T) {
	topo := &spec.Specification{
		Grafanas:      []*spec.GrafanaSpec{{Host: "grafana", Port: 3000, DeployDir: "/srv/grafana"}},
		Monitors:      []*spec.PrometheusSpec{{Host: "prometheus", Port: 9090, DeployDir: "/srv/prometheus"}},
		Alertmanagers: []*spec.AlertmanagerSpec{{Host: "alertmanager", WebPort: 9093, DeployDir: "/srv/alertmanager"}},
	}
	for _, component := range []string{spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertmanager} {
		t.Run(component, func(t *testing.T) {
			instance := spec.FindComponent(topo, component).Instances()[0]
			op, err := updateTopologyOp(instance)
			require.NoError(t, err)
			require.True(t, op.IsPut())
			require.Equal(t, "/topology/"+component, string(op.KeyBytes()))
			var topology componentTopology
			require.NoError(t, json.Unmarshal(op.ValueBytes(), &topology))
			require.Equal(t, componentTopology{IP: instance.GetHost(), Port: instance.GetPort(), DeployPath: instance.DeployDir()}, topology)
		})
	}
}
