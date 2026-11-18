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

package manager

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
)

// DetachGrafana transfers the specified Grafana instances out of TiUP
// management. It is metadata-only: no node is contacted, services keep running
// and remote files are left untouched; the operator takes over their whole
// lifecycle. There is no reattach command.
//
// The command proceeds in five steps: validate input and load the cluster
// metadata, resolve the target instances, verify directory safety against the
// post-detach topology, present the consequences and confirm, then persist
// the new metadata.
func (m *Manager) DetachGrafana(name string, nodes []string, skipConfirm bool) error {
	// Validate input and load the cluster metadata.
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
		return err
	}
	metadata, err := m.meta(name)
	if err != nil {
		return err
	}
	clusterMeta, ok := metadata.(*spec.ClusterMeta)
	if !ok {
		return errors.New("detach-grafana only supports TiDB clusters")
	}
	if clusterMeta.Topology == nil {
		return errors.Errorf("cluster %q has no topology in its metadata", name)
	}
	if len(nodes) == 0 {
		return errors.New("specify at least one Grafana instance with --node (-N)")
	}

	// Resolve -N values to the Grafana instances to detach, rejecting unknown
	// IDs, non-Grafana nodes and duplicates. targets and targetIDs run in
	// parallel: targets[i].ID() == targetIDs[i].
	instanceByID := make(map[string]spec.Instance)
	clusterMeta.Topology.IterInstance(func(instance spec.Instance) {
		instanceByID[instance.ID()] = instance
	})
	var targets []spec.Instance
	var targetIDs []string
	seen := set.NewStringSet()
	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node == "" {
			return errors.New("Grafana node IDs must not be empty")
		}
		instance, found := instanceByID[node]
		if !found {
			return errors.Errorf("node %q does not exist in cluster %q", node, name)
		}
		if instance.ComponentName() != spec.ComponentGrafana {
			return errors.Errorf("node %q is %s, not Grafana", node, instance.ComponentName())
		}
		if seen.Exist(node) {
			continue
		}
		seen.Insert(node)
		targets = append(targets, instance)
		targetIDs = append(targetIDs, node)
	}

	// Validate directory safety against the topology that will remain. The
	// check cannot wait: once the Grafana entries leave the metadata,
	// CountDir-based validation no longer sees their directories, and an
	// overlap with a remaining (e.g. parent) directory would never be
	// reported again. postDetach is a throwaway view built for this check
	// (and the warnings below); clusterMeta itself is never modified here —
	// only the UpdateMeta task at the end persists a change.
	postDetach := postDetachMeta(clusterMeta, targetIDs)
	if err := validateGrafanaDetachDirs(m.specManager, name, postDetach, targets); err != nil {
		return err
	}

	// Present the handoff summary and its consequences, then confirm.
	m.logger.Warnf("Detach Grafana from cluster %s: %s", name, color.HiYellowString(strings.Join(targetIDs, ", ")))
	for _, instance := range targets {
		m.logger.Infof("  %s: %s", instance.ID(), spec.Abs(clusterMeta.User, instance.DeployDir()))
	}
	m.logger.Warnf(`%s
Services and remote files are kept as-is: you take responsibility for their upgrades,
backups, configuration, dashboards, datasources and health monitoring, and TiUP will no
longer reserve their ports, directories or service names.
The directory check only covers paths recorded in this local cluster profile; verify the
actual layout, environment overrides, symlinks and mounts yourself.
Finish all other TiUP changes before proceeding: detach does not lock out concurrent operations.`,
		color.HiRedString("There is no supported reattach command."))
	// A target host that loses its last managed component in the remaining
	// topology stops receiving monitoring too; its exporters may keep
	// running without an owner.
	remainingHosts, _ := getMonitorHosts(postDetach.Topology)
	orphanedHosts := set.NewStringSet()
	for _, instance := range targets {
		if _, found := remainingHosts[instance.GetManageHost()]; !found {
			orphanedHosts.Insert(instance.GetManageHost())
		}
	}
	if len(orphanedHosts) > 0 {
		hosts := orphanedHosts.Slice()
		sort.Strings(hosts)
		m.logger.Warnf("This cluster will stop managing and monitoring host(s) %s. Exporters may remain running; check for another manager before taking over or removing them.", color.HiYellowString(strings.Join(hosts, ", ")))
	}
	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError("Transfer Grafana management to you? [y/N]: "); err != nil {
			return err
		}
	}

	// Persist the change: UpdateMeta rebuilds the metadata without the
	// detached instances and saves it (with a backup of the previous file);
	// the in-memory clusterMeta is not modified.
	ctx := ctxt.New(context.Background(), 1, m.logger)
	if err := task.NewBuilder(m.logger).UpdateMeta(name, clusterMeta, targetIDs).Build().Execute(ctx); err != nil {
		return errors.Annotate(err, "failed to save detach metadata; inspect meta.yaml and backup/ before retrying")
	}
	m.logger.Infof("Detached Grafana instance(s) %s from cluster %s. No remote service or file was changed.", strings.Join(targetIDs, ", "), name)
	m.logger.Warnf("%s", grafanaDetachFollowUpWarning(postDetach.Topology))
	return nil
}

// postDetachMeta returns a throwaway view of metadata with the given Grafana
// instances already removed, for validating the topology that will remain.
// It must never be persisted: both struct copies are shallow, so apart from
// Topology (and Topology.Grafanas) every field still aliases the loaded
// metadata.
func postDetachMeta(metadata *spec.ClusterMeta, detachIDs []string) *spec.ClusterMeta {
	detaching := set.NewStringSet(detachIDs...)
	topology := *metadata.Topology
	topology.Grafanas = nil
	// GrafanaComponent.Instances() yields instances in Grafanas order, so the
	// i-th instance corresponds to Grafanas[i] in the original topology.
	for i, instance := range (&spec.GrafanaComponent{Topology: metadata.Topology}).Instances() {
		if !detaching.Exist(instance.ID()) {
			topology.Grafanas = append(topology.Grafanas, metadata.Topology.Grafanas[i])
		}
	}
	view := *metadata
	view.Topology = &topology
	return &view
}

// grafanaDetachFollowUpWarning describes how monitoring behaves after the
// handoff. Like the dashboard warning printed after scale-in, it is shown
// once the change is persisted: it explains the world afterwards rather than
// something the operator must weigh before confirming.
func grafanaDetachFollowUpWarning(topology *spec.Specification) string {
	var msg strings.Builder
	if len(topology.Grafanas) > 0 {
		fmt.Fprintf(&msg, "%d managed Grafana instance(s) remain; future topology refreshes will use their monitoring entry.\n", len(topology.Grafanas))
	} else {
		msg.WriteString("Existing Grafana discovery information is kept, but is not checked or recreated. Adding managed Grafana later will replace it.\n")
	}
	msg.WriteString("Future Prometheus configuration refreshes will remove health probes for the detached Grafana instances.")
	return color.YellowString("\n%s", msg.String())
}
