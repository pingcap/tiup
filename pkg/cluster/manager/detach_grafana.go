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
	"sort"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
)

// DetachGrafana transfers specified Grafana instances out of TiUP management.
// It only updates local metadata; it does not connect to cluster nodes or PD.
func (m *Manager) DetachGrafana(name string, nodes []string, skipConfirm bool) error {
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

	instances := make(map[string]spec.Instance)
	clusterMeta.Topology.IterInstance(func(instance spec.Instance) {
		instances[instance.ID()] = instance
	})
	selected := set.NewStringSet()
	var targets []spec.Instance
	var nodeIDs []string
	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node == "" {
			return errors.New("Grafana node IDs must not be empty")
		}
		instance, found := instances[node]
		if !found {
			return errors.Errorf("node %q does not exist in cluster %q", node, name)
		}
		if instance.ComponentName() != spec.ComponentGrafana {
			return errors.Errorf("node %q is %s, not Grafana", node, instance.ComponentName())
		}
		if !selected.Exist(node) {
			selected.Insert(node)
			targets = append(targets, instance)
			nodeIDs = append(nodeIDs, node)
		}
	}

	// Check the topology that will remain without mutating the loaded metadata.
	// Removing a nested Grafana also removes CountDir's protection of its parents.
	remaining := *clusterMeta.Topology
	remaining.Grafanas = nil
	for i, instance := range (&spec.GrafanaComponent{Topology: clusterMeta.Topology}).Instances() {
		if !selected.Exist(instance.ID()) {
			remaining.Grafanas = append(remaining.Grafanas, clusterMeta.Topology.Grafanas[i])
		}
	}
	remainingMeta := *clusterMeta
	remainingMeta.Topology = &remaining
	if err := m.validateGrafanaDetachDirs(name, &remainingMeta, targets); err != nil {
		return err
	}

	m.logger.Warnf("Detach Grafana from cluster %s: %s", name, strings.Join(nodeIDs, ", "))
	for _, instance := range targets {
		m.logger.Infof("  %s: %s", instance.ID(), spec.Abs(clusterMeta.User, instance.DeployDir()))
	}
	m.logger.Warnf("Complete all other TiUP changes before proceeding; detach does not lock out concurrent operations.")
	m.logger.Warnf("Services and remote files will be kept. You take responsibility for Grafana upgrades, backups, configuration, dashboards, datasources and health monitoring.")
	m.logger.Warnf("The directory check only covers recorded paths in this local cluster profile. Verify actual paths, environment overrides, symlinks and mounts separately.")
	m.logger.Warnf("There is no supported reattach command, and ports, directories and service names will no longer be reserved by TiUP.")
	if len(remaining.Grafanas) > 0 {
		m.logger.Warnf("%d managed Grafana instance(s) remain; future topology refreshes will use their monitoring entry.", len(remaining.Grafanas))
	} else {
		m.logger.Warnf("Existing Grafana discovery information is kept, but is not checked or recreated. Adding managed Grafana later will replace it.")
	}
	remainingHosts, _ := getMonitorHosts(&remaining)
	orphanedHosts := set.NewStringSet()
	for _, instance := range targets {
		if _, found := remainingHosts[instance.GetManageHost()]; !found {
			orphanedHosts.Insert(instance.GetManageHost())
		}
	}
	if len(orphanedHosts) > 0 {
		hosts := orphanedHosts.Slice()
		sort.Strings(hosts)
		m.logger.Warnf("This cluster will stop managing and monitoring host(s) %s. Exporters may remain running; check for another manager before taking over or removing them.", strings.Join(hosts, ", "))
	}
	m.logger.Warnf("Future Prometheus configuration refreshes will remove health probes for the detached Grafana instances.")
	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError("Transfer Grafana management to you? [y/N]: "); err != nil {
			return err
		}
	}

	ctx := ctxt.New(context.Background(), 1, m.logger)
	if err := task.NewBuilder(m.logger).UpdateMeta(name, clusterMeta, nodeIDs).Build().Execute(ctx); err != nil {
		return errors.Annotate(err, "failed to save detach metadata; inspect meta.yaml and backup/ before retrying")
	}
	m.logger.Infof("Detached Grafana instance(s) %s from cluster %s. No remote service or file was changed.", strings.Join(nodeIDs, ", "), name)
	return nil
}
