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

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/set"
)

// UpdateMeta is used to maintain the cluster meta information
type UpdateMeta struct {
	cluster        string
	metadata       *spec.ClusterMeta
	deletedNodeIDs []string
}

// Execute implements the Task interface
// the metadata especially the topology is in wide use,
// the other callers point to this field by a pointer,
// so we should update the original topology directly, and don't make a copy
func (u *UpdateMeta) Execute(ctx context.Context) error {
	deleted := set.NewStringSet(u.deletedNodeIDs...)
	topo := u.metadata.Topology

	newMeta := &spec.ClusterMeta{}
	*newMeta = *u.metadata
	newMeta.Topology = &spec.Specification{
		GlobalOptions:    u.metadata.Topology.GlobalOptions,
		MonitoredOptions: u.metadata.Topology.MonitoredOptions,
		ServerConfigs:    u.metadata.Topology.ServerConfigs,
	}

	tidbServers := make([]*spec.TiDBSpec, 0)
	for i, instance := range (&spec.TiDBComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tidbServers = append(tidbServers, topo.TiDBServers[i])
	}
	newMeta.Topology.TiDBServers = tidbServers

	tikvServers := make([]*spec.TiKVSpec, 0)
	for i, instance := range (&spec.TiKVComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tikvServers = append(tikvServers, topo.TiKVServers[i])
	}
	newMeta.Topology.TiKVServers = tikvServers

	pdServers := make([]*spec.PDSpec, 0)
	for i, instance := range (&spec.PDComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		pdServers = append(pdServers, topo.PDServers[i])
	}
	newMeta.Topology.PDServers = pdServers

	tsoServers := make([]*spec.TSOSpec, 0)
	for i, instance := range (&spec.TSOComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tsoServers = append(tsoServers, topo.TSOServers[i])
	}
	newMeta.Topology.TSOServers = tsoServers

	schedulingServers := make([]*spec.SchedulingSpec, 0)
	for i, instance := range (&spec.SchedulingComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		schedulingServers = append(schedulingServers, topo.SchedulingServers[i])
	}
	newMeta.Topology.SchedulingServers = schedulingServers

	tiproxyServers := make([]*spec.TiProxySpec, 0)
	for i, instance := range (&spec.TiProxyComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tiproxyServers = append(tiproxyServers, topo.TiProxyServers[i])
	}
	newMeta.Topology.TiProxyServers = tiproxyServers

	dashboardServers := make([]*spec.DashboardSpec, 0)
	for i, instance := range (&spec.DashboardComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		dashboardServers = append(dashboardServers, topo.DashboardServers[i])
	}
	newMeta.Topology.DashboardServers = dashboardServers

	tiflashServers := make([]*spec.TiFlashSpec, 0)
	for i, instance := range (&spec.TiFlashComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tiflashServers = append(tiflashServers, topo.TiFlashServers[i])
	}
	newMeta.Topology.TiFlashServers = tiflashServers

	pumpServers := make([]*spec.PumpSpec, 0)
	for i, instance := range (&spec.PumpComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		pumpServers = append(pumpServers, topo.PumpServers[i])
	}
	newMeta.Topology.PumpServers = pumpServers

	drainerServers := make([]*spec.DrainerSpec, 0)
	for i, instance := range (&spec.DrainerComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		drainerServers = append(drainerServers, topo.Drainers[i])
	}
	newMeta.Topology.Drainers = drainerServers

	cdcServers := make([]*spec.CDCSpec, 0)
	for i, instance := range (&spec.CDCComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		cdcServers = append(cdcServers, topo.CDCServers[i])
	}
	newMeta.Topology.CDCServers = cdcServers

	tikvCDCServers := make([]*spec.TiKVCDCSpec, 0)
	for i, instance := range (&spec.TiKVCDCComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tikvCDCServers = append(tikvCDCServers, topo.TiKVCDCServers[i])
	}
	newMeta.Topology.TiKVCDCServers = tikvCDCServers

	tisparkWorkers := make([]*spec.TiSparkWorkerSpec, 0)
	for i, instance := range (&spec.TiSparkWorkerComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tisparkWorkers = append(tisparkWorkers, topo.TiSparkWorkers[i])
	}
	newMeta.Topology.TiSparkWorkers = tisparkWorkers

	tisparkMasters := make([]*spec.TiSparkMasterSpec, 0)
	for i, instance := range (&spec.TiSparkMasterComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		tisparkMasters = append(tisparkMasters, topo.TiSparkMasters[i])
	}
	newMeta.Topology.TiSparkMasters = tisparkMasters

	monitors := make([]*spec.PrometheusSpec, 0)
	for i, instance := range (&spec.MonitorComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		monitors = append(monitors, topo.Monitors[i])
	}
	newMeta.Topology.Monitors = monitors

	grafanas := make([]*spec.GrafanaSpec, 0)
	for i, instance := range (&spec.GrafanaComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		grafanas = append(grafanas, topo.Grafanas[i])
	}
	newMeta.Topology.Grafanas = grafanas

	alertmanagers := make([]*spec.AlertmanagerSpec, 0)
	for i, instance := range (&spec.AlertManagerComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		alertmanagers = append(alertmanagers, topo.Alertmanagers[i])
	}
	newMeta.Topology.Alertmanagers = alertmanagers

	return spec.SaveClusterMeta(u.cluster, newMeta)
}

// Rollback implements the Task interface
func (u *UpdateMeta) Rollback(ctx context.Context) error {
	return spec.SaveClusterMeta(u.cluster, u.metadata)
}

// String implements the fmt.Stringer interface
func (u *UpdateMeta) String() string {
	return fmt.Sprintf("UpdateMeta: cluster=%s, deleted=`'%s'`", u.cluster, strings.Join(u.deletedNodeIDs, "','"))
}
