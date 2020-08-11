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

	dmspec "github.com/pingcap/tiup/components/dm/spec"

	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
)

// UpdateDMMeta is used to maintain the DM meta information
type UpdateDMMeta struct {
	cluster        string
	metadata       *dmspec.Metadata
	deletedNodesID []string
}

// NewUpdateDMMeta create i update dm meta task.
func NewUpdateDMMeta(cluster string, metadata *dmspec.Metadata, deletedNodesID []string) *UpdateDMMeta {
	return &UpdateDMMeta{
		cluster:        cluster,
		metadata:       metadata,
		deletedNodesID: deletedNodesID,
	}
}

// Execute implements the Task interface
func (u *UpdateDMMeta) Execute(ctx *task.Context) error {
	// make a copy
	newMeta := &dmspec.Metadata{}
	*newMeta = *u.metadata
	newMeta.Topology = &dmspec.Topology{
		GlobalOptions: u.metadata.Topology.GlobalOptions,
		// MonitoredOptions: u.metadata.Topology.MonitoredOptions,
		ServerConfigs: u.metadata.Topology.ServerConfigs,
	}

	deleted := set.NewStringSet(u.deletedNodesID...)
	topo := u.metadata.Topology
	for i, instance := range (&dmspec.DMMasterComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Masters = append(newMeta.Topology.Masters, topo.Masters[i])
	}
	for i, instance := range (&dmspec.DMWorkerComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Workers = append(newMeta.Topology.Workers, topo.Workers[i])
	}
	for i, instance := range (&dmspec.MonitorComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Monitors = append(newMeta.Topology.Monitors, topo.Monitors[i])
	}
	for i, instance := range (&dmspec.GrafanaComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Grafana = append(newMeta.Topology.Grafana, topo.Grafana[i])
	}
	for i, instance := range (&dmspec.AlertManagerComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Alertmanager = append(newMeta.Topology.Alertmanager, topo.Alertmanager[i])
	}

	return dmspec.GetSpecManager().SaveMeta(u.cluster, newMeta)
}

// Rollback implements the Task interface
func (u *UpdateDMMeta) Rollback(ctx *task.Context) error {
	return dmspec.GetSpecManager().SaveMeta(u.cluster, u.metadata)
}

// String implements the fmt.Stringer interface
func (u *UpdateDMMeta) String() string {
	return fmt.Sprintf("UpdateMeta: cluster=%s, deleted=`'%s'`", u.cluster, strings.Join(u.deletedNodesID, "','"))
}
