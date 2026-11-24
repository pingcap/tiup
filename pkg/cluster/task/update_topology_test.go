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
	"encoding/json"
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestUpdateInstancesAndOps(t *testing.T) {
	for _, component := range []string{spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertmanager} {
		for _, test := range []struct {
			name     string
			empty    bool
			deleted  []string
			detached []spec.DetachedGrafana // detached-grafana records kept in the metadata
			entry    *componentTopology     // current value of the grafana discovery entry
			preserve bool                   // a grafana entry it vouches for must survive
			wantIDs  []string
		}{
			// An empty surviving list without detached records deletes the
			// entry: after a scale-in whose PD cleanup failed midway, the
			// saved metadata no longer lists the Grafana, and only this
			// delete — retried by every later refresh — converges the stale
			// entry away. This is the behavior from before detach-grafana
			// existed and must survive it.
			{name: "empty", empty: true},
			{name: "empty with unrelated deletion", empty: true, deleted: []string{"other:1234"}},
			// A detached record preserves the entry only while the entry
			// points at the address the record vouches for: the entry
			// belongs to an instance TiUP handed over.
			{name: "entry at a detached address is preserved", empty: true,
				detached: []spec.DetachedGrafana{{Host: "gone", Port: 3000}},
				entry:    &componentTopology{IP: "gone", Port: 3000}, preserve: true},
			// The record vouches for its address, not for whatever the
			// entry happens to hold: a refresh rewrote the entry to the
			// surviving managed instance, and scaling that one in leaves a
			// dead address no record covers — delete it, converging as
			// before detach-grafana existed.
			{name: "entry at the removed managed address is deleted", empty: true,
				detached: []spec.DetachedGrafana{{Host: "gone", Port: 3000}},
				entry:    &componentTopology{IP: "second", Port: 3000}},
			// A missing entry stays missing: nothing writes one back on
			// behalf of an instance TiUP no longer manages.
			{name: "absent entry stays absent", empty: true,
				detached: []spec.DetachedGrafana{{Host: "gone", Port: 3000}}, preserve: true},
			// Hosts compare by address, so compressed and expanded
			// spellings of one IPv6 address are the same machine.
			{name: "IPv6 spelling differences preserve the entry", empty: true,
				detached: []spec.DetachedGrafana{{Host: "2001:db8::1", Port: 3000}},
				entry:    &componentTopology{IP: "2001:db8:0:0:0:0:0:1", Port: 3000}, preserve: true},
			// Scaling in the last managed Grafana runs this update with the
			// removed instance still in the in-memory list; a detached
			// record must suppress that in-chain deletion too when the entry
			// is the one it vouches for, or the entry preserved for the
			// handed-over instance is lost to the unrelated scale-in.
			{name: "in-chain deletion of an entry at a detached address is suppressed", deleted: []string{"first:3000", "second:3000"},
				detached: []spec.DetachedGrafana{{Host: "gone", Port: 3000}},
				entry:    &componentTopology{IP: "gone", Port: 3000}, preserve: true},
			// The same in-chain deletion with the entry pointing at the
			// removed managed instance is the ordinary scale-in: delete.
			{name: "in-chain deletion of the removed managed address proceeds", deleted: []string{"first:3000", "second:3000"},
				detached: []spec.DetachedGrafana{{Host: "gone", Port: 3000}},
				entry:    &componentTopology{IP: "second", Port: 3000}},
			{name: "all retained", wantIDs: []string{"first:3000", "second:3000"}},
			{name: "first deleted", deleted: []string{"first:3000"}, wantIDs: []string{"second:3000"}},
			{name: "all deleted", deleted: []string{"first:3000", "second:3000"}},
			{name: "unrelated deletion", deleted: []string{"other:1234"}, wantIDs: []string{"first:3000", "second:3000"}},
		} {
			t.Run(component+"/"+test.name, func(t *testing.T) {
				topo := &spec.Specification{
					Grafanas: []*spec.GrafanaSpec{
						{Host: "first", Port: 3000}, {Host: "second", Port: 3000},
					},
					Monitors: []*spec.PrometheusSpec{
						{Host: "first", Port: 3000}, {Host: "second", Port: 3000},
					},
					Alertmanagers: []*spec.AlertmanagerSpec{
						{Host: "first", WebPort: 3000}, {Host: "second", WebPort: 3000},
					},
				}
				for _, detached := range test.detached {
					topo.DetachedGrafanas = append(topo.DetachedGrafanas, &detached)
				}
				instances := spec.FindComponent(topo, component).Instances()
				if test.empty {
					instances = nil
				}
				previous := (&spec.TiDBComponent{Topology: &spec.Specification{
					TiDBServers: []*spec.TiDBSpec{{Host: "tidb", Port: 4000}},
				}}).Instances()
				var grafanaEntry *componentTopology
				if component == spec.ComponentGrafana {
					grafanaEntry = test.entry
				}
				ops, retained := updateInstancesAndOps(
					[]clientv3.Op{clientv3.OpPut("/previous", "untouched")}, previous,
					set.NewStringSet(test.deleted...), instances, component,
					topo.DetachedGrafanas, grafanaEntry,
				)

				require.True(t, ops[0].IsPut())
				require.Equal(t, "/previous", string(ops[0].KeyBytes()))
				require.Equal(t, "untouched", string(ops[0].ValueBytes()))
				require.Same(t, previous[0], retained[0])
				var ids []string
				for _, instance := range retained[1:] {
					ids = append(ids, instance.ID())
				}
				require.Equal(t, test.wantIDs, ids)
				// The discovery entry survives an empty surviving list only
				// for Grafana and only while a detached record vouches for
				// the address it holds; every other empty list deletes the
				// entry.
				wantDelete := len(test.wantIDs) == 0 && !(component == spec.ComponentGrafana && test.preserve)
				if wantDelete {
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
