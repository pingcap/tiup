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
				instances := spec.FindComponent(topo, component).Instances()
				if test.empty {
					instances = nil
				}
				previous := (&spec.TiDBComponent{Topology: &spec.Specification{
					TiDBServers: []*spec.TiDBSpec{{Host: "tidb", Port: 4000}},
				}}).Instances()
				ops, retained := updateInstancesAndOps(
					[]clientv3.Op{clientv3.OpPut("/previous", "untouched")}, previous,
					set.NewStringSet(test.deleted...), instances, component,
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
				wantDelete := len(test.wantIDs) == 0 && (component != spec.ComponentGrafana || !test.empty)
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
