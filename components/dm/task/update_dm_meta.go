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
	"context"
	"fmt"
	"strings"

	dmspec "github.com/pingcap/tiup/components/dm/spec"

	"github.com/pingcap/tiup/pkg/cluster/spec"
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
// the metadata especially the topology is in wide use,
// the other callers point to this field by a pointer,
// so we should update the original topology directly, and don't make a copy
func (u *UpdateDMMeta) Execute(ctx context.Context) error {
	deleted := set.NewStringSet(u.deletedNodesID...)
	topo := u.metadata.Topology
	masters := make([]*dmspec.MasterSpec, 0)
	for i, instance := range (&dmspec.DMMasterComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		masters = append(masters, topo.Masters[i])
	}
	topo.Masters = masters

	workers := make([]*dmspec.WorkerSpec, 0)
	for i, instance := range (&dmspec.DMWorkerComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		workers = append(workers, topo.Workers[i])
	}
	topo.Workers = workers

	monitors := make([]*spec.PrometheusSpec, 0)
	for i, instance := range (&spec.MonitorComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		monitors = append(monitors, topo.Monitors[i])
	}
	topo.Monitors = monitors

	grafanas := make([]*spec.GrafanaSpec, 0)
	for i, instance := range (&spec.GrafanaComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		grafanas = append(grafanas, topo.Grafanas[i])
	}
	topo.Grafanas = grafanas

	alertmanagers := make([]*spec.AlertmanagerSpec, 0)
	for i, instance := range (&spec.AlertManagerComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		alertmanagers = append(alertmanagers, topo.Alertmanagers[i])
	}
	topo.Alertmanagers = alertmanagers

	return dmspec.GetSpecManager().SaveMeta(u.cluster, u.metadata)
}

// Rollback implements the Task interface
func (u *UpdateDMMeta) Rollback(ctx context.Context) error {
	return dmspec.GetSpecManager().SaveMeta(u.cluster, u.metadata)
}

// String implements the fmt.Stringer interface
func (u *UpdateDMMeta) String() string {
	return fmt.Sprintf("UpdateMeta: cluster=%s, deleted=`'%s'`", u.cluster, strings.Join(u.deletedNodesID, "','"))
}
