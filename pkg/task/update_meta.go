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
	"fmt"
	"strings"

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
	for i, instance := range (&meta.TiDBComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.TiDBServers = append(newMeta.Topology.TiDBServers, topo.TiDBServers[i])
	}
	for i, instance := range (&meta.TiKVComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.TiKVServers = append(newMeta.Topology.TiKVServers, topo.TiKVServers[i])
	}
	for i, instance := range (&meta.PDComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.PDServers = append(newMeta.Topology.PDServers, topo.PDServers[i])
	}
	for i, instance := range (&meta.PumpComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.PumpServers = append(newMeta.Topology.PumpServers, topo.PumpServers[i])
	}
	for i, instance := range (&meta.DrainerComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Drainers = append(newMeta.Topology.Drainers, topo.Drainers[i])
	}
	for i, instance := range (&meta.MonitorComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Monitors = append(newMeta.Topology.Monitors, topo.Monitors[i])
	}
	for i, instance := range (&meta.GrafanaComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Grafana = append(newMeta.Topology.Grafana, topo.Grafana[i])
	}
	for i, instance := range (&meta.AlertManagerComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Alertmanager = append(newMeta.Topology.Alertmanager, topo.Alertmanager[i])
	}
	return meta.SaveClusterMeta(u.cluster, newMeta)
}

// Rollback implements the Task interface
func (u *UpdateMeta) Rollback(ctx *Context) error {
	return meta.SaveClusterMeta(u.cluster, u.metadata)
}

// String implements the fmt.Stringer interface
func (u *UpdateMeta) String() string {
	return fmt.Sprintf("UpdateMeta: cluster=%s, deleted=`'%s'`", u.cluster, strings.Join(u.deletedNodesID, "','"))
}
