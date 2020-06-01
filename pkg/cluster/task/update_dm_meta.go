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

	"github.com/pingcap/tiup/pkg/cluster/meta"

	"github.com/pingcap/tiup/pkg/set"
)

// UpdateDMMeta is used to maintain the DM meta information
type UpdateDMMeta struct {
	cluster        string
	metadata       *meta.DMMeta
	deletedNodesID []string
}

// Execute implements the Task interface
func (u *UpdateDMMeta) Execute(ctx *Context) error {
	// make a copy
	newMeta := &meta.DMMeta{}
	*newMeta = *u.metadata
	newMeta.Topology = &meta.DMTopologySpecification{
		GlobalOptions:    u.metadata.Topology.GlobalOptions,
		MonitoredOptions: u.metadata.Topology.MonitoredOptions,
		ServerConfigs:    u.metadata.Topology.ServerConfigs,
	}

	deleted := set.NewStringSet(u.deletedNodesID...)
	topo := u.metadata.Topology
	for i, instance := range (&meta.DMMasterComponent{DMSpecification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Masters = append(newMeta.Topology.Masters, topo.Masters[i])
	}
	for i, instance := range (&meta.DMWorkerComponent{DMSpecification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Workers = append(newMeta.Topology.Workers, topo.Workers[i])
	}
	for i, instance := range (&meta.DMPortalComponent{DMSpecification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			continue
		}
		newMeta.Topology.Portals = append(newMeta.Topology.Portals, topo.Portals[i])
	}

	return meta.SaveDMMeta(u.cluster, newMeta)
}

// Rollback implements the Task interface
func (u *UpdateDMMeta) Rollback(ctx *Context) error {
	return meta.SaveDMMeta(u.cluster, u.metadata)
}

// String implements the fmt.Stringer interface
func (u *UpdateDMMeta) String() string {
	return fmt.Sprintf("UpdateMeta: cluster=%s, deleted=`'%s'`", u.cluster, strings.Join(u.deletedNodesID, "','"))
}
