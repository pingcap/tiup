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
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/stretchr/testify/require"
)

func TestUpdateMetaKeepsDetachedGrafanas(t *testing.T) {
	// A scale-in of unrelated components persists the metadata through
	// UpdateMeta, which rebuilds the topology field by field; dropping the
	// detached records there would make the next refresh treat the
	// handed-over Grafana's entry as stale and delete it — the very loss the
	// records exist to prevent.
	t.Setenv("TIUP_COMPONENT_DATA_DIR", t.TempDir())
	require.NoError(t, spec.Initialize("cluster"))
	metadata := &spec.ClusterMeta{
		User: "tidb",
		Topology: &spec.Specification{
			TiDBServers: []*spec.TiDBSpec{
				{Host: "n1", Port: 4000}, {Host: "n2", Port: 4000},
			},
			DetachedGrafanas: []*spec.DetachedGrafana{{Host: "gone", Port: 3000}},
		},
	}

	updateMeta := &UpdateMeta{cluster: "test", metadata: metadata, deletedNodeIDs: []string{"n2:4000"}}
	require.NoError(t, updateMeta.Execute(context.Background()))

	saved, err := spec.ClusterMetadata("test")
	require.NoError(t, err)
	require.Equal(t, []*spec.DetachedGrafana{{Host: "gone", Port: 3000}}, saved.Topology.DetachedGrafanas)
	require.Len(t, saved.Topology.TiDBServers, 1)
}
