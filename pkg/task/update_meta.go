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

package task

import (
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/set"
)

// UpdateMeta is used to maintain the cluster meta information
type UpdateMeta struct {
	cluster        string
	metadata       *meta.ClusterMeta
	deletedNodesID []string
}

// Execute implements the Task interface
func (u *UpdateMeta) Execute(ctx *Context) error {
	// make a copy
	newMeta := &meta.ClusterMeta{}
	*newMeta = *u.metadata
	newMeta.Topology = &meta.TopologySpecification{}

	deleted := set.NewStringSet(u.deletedNodesID...)
	topo := u.metadata.Topology
	for _, instance := range topo.TiDBServers {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.TiDBServers = append(newMeta.Topology.TiDBServers, instance)
	}
	for _, instance := range topo.TiKVServers {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.TiKVServers = append(newMeta.Topology.TiKVServers, instance)
	}
	for _, instance := range topo.PDServers {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.PDServers = append(newMeta.Topology.PDServers, instance)
	}
	for _, instance := range topo.PumpServers {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.PumpServers = append(newMeta.Topology.PumpServers, instance)
	}
	for _, instance := range topo.Drainers {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.Drainers = append(newMeta.Topology.Drainers, instance)
	}
	for _, instance := range topo.MonitorSpec {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.MonitorSpec = append(newMeta.Topology.MonitorSpec, instance)
	}
	for _, instance := range topo.Grafana {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.Grafana = append(newMeta.Topology.Grafana, instance)
	}
	for _, instance := range topo.Alertmanager {
		if deleted.Exist(instance.UUID) {
			continue
		}
		newMeta.Topology.Alertmanager = append(newMeta.Topology.Alertmanager, instance)
	}

	return meta.SaveClusterMeta(u.cluster, newMeta)
}

// Rollback implements the Task interface
func (u *UpdateMeta) Rollback(ctx *Context) error {
	return meta.SaveClusterMeta(u.cluster, u.metadata)
}
