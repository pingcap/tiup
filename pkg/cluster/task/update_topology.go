package task

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// UpdateTopology is used to maintain the cluster meta information
type UpdateTopology struct {
	cluster        string
	profileDir     string
	metadata       *spec.ClusterMeta
	deletedNodeIDs []string
	tcpProxy       *proxy.TCPProxy
}

// String implements the fmt.Stringer interface
func (u *UpdateTopology) String() string {
	return fmt.Sprintf("UpdateTopology: cluster=%s", u.cluster)
}

// Execute implements the Task interface
func (u *UpdateTopology) Execute(ctx context.Context) error {
	tlsCfg, err := u.metadata.Topology.TLSConfig(
		filepath.Join(u.profileDir, spec.TLSCertKeyDir),
	)
	if err != nil {
		return err
	}
	var client *clientv3.Client
	if u.tcpProxy == nil {
		client, err = u.metadata.Topology.GetEtcdClient(tlsCfg)
	} else {
		var closeC chan struct{}
		client, closeC, err = u.metadata.Topology.GetEtcdProxyClient(tlsCfg, u.tcpProxy)
		defer u.tcpProxy.Close(closeC)
	}
	if err != nil {
		return err
	}
	// fix https://github.com/pingcap/tiup/issues/333
	// etcd client defaults to wait forever
	// if all pd were down, don't hang forever
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	txn := client.Txn(ctx)

	topo := u.metadata.Topology

	deleted := set.NewStringSet(u.deletedNodeIDs...)

	var ops []clientv3.Op
	var instances []spec.Instance

	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.MonitorComponent{Topology: topo}).Instances(), "prometheus")
	// The entry is read from etcd, and the metadata read back, only while a
	// cleanup is owed. updateGrafanaAndOps decides both and hands back the
	// entry to delete, the only one this refresh may delete.
	var owed bool
	var cleanup *topologyEntry
	ops, instances, owed, cleanup, err = updateGrafanaAndOps(ops, instances, deleted, (&spec.GrafanaComponent{Topology: topo}).Instances(),
		topo.PendingGrafanaEntryCleanups, func() (*topologyEntry, error) {
			return readTopologyEntry(ctx, client, spec.ComponentGrafana)
		})
	if err != nil {
		return err
	}
	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.AlertManagerComponent{Topology: topo}).Instances(), "alertmanager")

	for _, instance := range (&spec.TiDBComponent{Topology: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			ops = append(ops, clientv3.OpDelete("/topology/tidb/"+utils.JoinHostPort(instance.GetHost(), instance.GetPort()), clientv3.WithPrefix()))
		}
	}

	// the prometheus,grafana,alertmanager stored in etcd will be used by other components (tidb, pd, etc.)
	// and they assume there is ONLY ONE prometheus.
	// ref https://github.com/pingcap/tiup/issues/954#issuecomment-737002185
	updated := set.NewStringSet()
	for _, ins := range instances {
		if updated.Exist(ins.ComponentName()) {
			continue
		}
		op, err := updateTopologyOp(ins)
		if err != nil {
			return err
		}

		updated.Insert(ins.ComponentName())
		ops = append(ops, *op)
	}

	if cleanup != nil {
		// Delete only the entry that was read. A key written since no longer
		// holds the leftover the decision matched and is left to whoever wrote
		// it. The other updates apply on either branch, so a failed comparison
		// cannot hold them back.
		key := "/topology/" + spec.ComponentGrafana
		txn = txn.
			If(clientv3.Compare(clientv3.ModRevision(key), "=", cleanup.modRevision)).
			Then(append(slices.Clone(ops), clientv3.OpDelete(key))...).
			Else(ops...)
	} else {
		txn = txn.Then(ops...)
	}
	if _, err := txn.Commit(); err != nil {
		return err
	}
	if !owed {
		return nil
	}
	return u.settleGrafanaEntryCleanups(ctx)
}

// componentTopology represent the topology info for alertmanager, prometheus and grafana.
type componentTopology struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	DeployPath string `json:"deploy_path"`
}

// topologyEntry is a discovery entry as etcd holds it: its value together with
// the revision that last wrote the key.
type topologyEntry struct {
	value       []byte
	modRevision int64
}

// updateInstancesAndOps receives alertmanager or prometheus instance list, if the list has
//
//	no member or all deleted, it will add a `OpDelete` in ops, otherwise it will push all current not deleted instances into instance list.
func updateInstancesAndOps(ops []clientv3.Op, ins []spec.Instance, deleted set.StringSet, instances []spec.Instance, componentName string) ([]clientv3.Op, []spec.Instance) {
	var currentInstances []spec.Instance
	for _, instance := range instances {
		if deleted.Exist(instance.ID()) {
			continue
		}
		currentInstances = append(currentInstances, instance)
	}

	if len(currentInstances) == 0 {
		ops = append(ops, clientv3.OpDelete("/topology/"+componentName))
	} else {
		ins = append(ins, currentInstances...)
	}
	return ops, ins
}

// updateGrafanaAndOps maintains the Grafana discovery entry (/topology/grafana
// in PD, where TiDB Dashboard reads the Grafana address) by one rule:
//
//	While TiUP manages a Grafana, the entry follows the managed instances.
//	Once it manages none, the entry is left alone, unread, unless it still
//	points at an instance TiUP removed and has not yet cleared the entry of:
//	that entry is deleted, and the deletion is retried until it succeeds.
//
// The second half comes down to ownership. Once TiUP manages no Grafana, the
// entry belongs to whoever runs Grafana: the last instance may have been
// handed over with detach-grafana and moved since, and its operator may have
// updated the entry. What TiUP wrote for an instance it removed is the one
// exception, as nothing listens at that address any more.
//
// TiUP can tell that exception only by remembering the removal, since the
// metadata no longer lists the instance afterwards. The removal is therefore
// recorded as a pending cleanup (spec.GrafanaEntryCleanup) until a refresh
// has dealt with the entry. An instance counts as removed if this run removes
// it (still in instances, listed in deleted) or a record says so, and
// settleGrafanaEntryCleanups drops the records once the refresh has committed.
// A missing entry stays missing.
//
// The entry to delete is returned rather than deleted here: Execute deletes
// it only while the key still carries the revision that was read, so that an
// entry rewritten in the meantime is left alone. The function also reports
// whether a cleanup is owed, for Execute to settle the record once the
// refresh has committed: the records the topology carried and the Grafana
// instances this run removes add up to what UpdateMeta has just saved, so the
// metadata on disk carries a record if, and only if, there is a cleanup.
func updateGrafanaAndOps(ops []clientv3.Op, ins []spec.Instance, deleted set.StringSet, instances []spec.Instance, recorded []*spec.GrafanaEntryCleanup, readEntry func() (*topologyEntry, error)) ([]clientv3.Op, []spec.Instance, bool, *topologyEntry, error) {
	var currentInstances []spec.Instance
	cleanups := append([]*spec.GrafanaEntryCleanup{}, recorded...)
	for _, instance := range instances {
		if !deleted.Exist(instance.ID()) {
			currentInstances = append(currentInstances, instance)
			continue
		}
		cleanups = append(cleanups, grafanaEntryCleanupFor(instance))
	}
	owed := len(cleanups) > 0

	if len(currentInstances) > 0 {
		return ops, append(ins, currentInstances...), owed, nil, nil
	}
	if !owed {
		return ops, ins, false, nil, nil
	}

	entry, err := readEntry()
	if err != nil {
		return ops, ins, false, nil, err
	}
	if entry != nil && grafanaEntryPointsAt(entry.value, cleanups) {
		return ops, ins, true, entry, nil
	}
	return ops, ins, true, nil, nil
}

// grafanaEntryCleanupFor returns the cleanup owed for a Grafana instance TiUP
// is removing.
func grafanaEntryCleanupFor(instance spec.Instance) *spec.GrafanaEntryCleanup {
	return &spec.GrafanaEntryCleanup{Host: instance.GetHost(), Port: instance.GetPort()}
}

// grafanaEntryPointsAt reports whether the discovery entry held in value was
// written for the Grafana instance at one of the addresses in cleanups. A
// missing value, or one not in the form TiUP writes, is not such a leftover.
func grafanaEntryPointsAt(value []byte, cleanups []*spec.GrafanaEntryCleanup) bool {
	if len(value) == 0 {
		return false
	}
	var entry componentTopology
	if err := json.Unmarshal(value, &entry); err != nil {
		return false
	}
	for _, cleanup := range cleanups {
		if cleanup.Matches(entry.IP, entry.Port) {
			return true
		}
	}
	return false
}

// readTopologyEntry returns a monitoring component's discovery entry together
// with the revision that last wrote it, or nil when the entry does not exist.
func readTopologyEntry(ctx context.Context, client *clientv3.Client, component string) (*topologyEntry, error) {
	resp, err := client.Get(ctx, "/topology/"+component)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return &topologyEntry{value: resp.Kvs[0].Value, modRevision: resp.Kvs[0].ModRevision}, nil
}

// settleGrafanaEntryCleanups drops the pending entry cleanups once this
// refresh has committed, as spec.GrafanaEntryCleanup describes. By then the
// entry a cleanup was waiting for is gone, whether the refresh deleted it or
// the key was written since the read and the delete was skipped. A record
// left behind would only let a later refresh delete an entry someone wrote at
// the removed instance's former address.
//
// The record is read back from disk, not from the in-memory topology: during a
// scale-in that one still lists the removed instance and predates the record
// UpdateMeta has just saved. It is cleared there as well, since a reload saves
// the in-memory topology back once the refresh is done.
//
// A metadata that cannot be read does not fail the refresh, which has
// committed already, and is not rewritten from what was read: commands such as
// scale-in and prune run on a topology that fails validation, so that an
// operator can repair it, and such a topology says nothing about the record.
// The record stays then, and the next refresh settles it.
func (u *UpdateTopology) settleGrafanaEntryCleanups(ctx context.Context) error {
	u.metadata.Topology.PendingGrafanaEntryCleanups = nil
	metadata, err := spec.ClusterMetadata(u.cluster)
	if err != nil {
		if logger, ok := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger); ok {
			logger.Debugf("The pending Grafana entry cleanups were not dropped, the next topology refresh retries: %v", err)
		}
		return nil
	}
	if metadata.Topology == nil || len(metadata.Topology.PendingGrafanaEntryCleanups) == 0 {
		return nil
	}
	metadata.Topology.PendingGrafanaEntryCleanups = nil
	return spec.SaveClusterMeta(u.cluster, metadata)
}

// updateTopologyOp receive an alertmanager, prometheus or grafana instance, and return an operation
//
//	for update it's topology.
func updateTopologyOp(instance spec.Instance) (*clientv3.Op, error) {
	switch compName := instance.ComponentName(); compName {
	case spec.ComponentAlertmanager, spec.ComponentPrometheus, spec.ComponentGrafana:
		topology := componentTopology{
			IP:         instance.GetHost(),
			Port:       instance.GetPort(),
			DeployPath: instance.DeployDir(),
		}
		data, err := json.Marshal(topology)
		if err != nil {
			return nil, err
		}
		op := clientv3.OpPut("/topology/"+compName, string(data))
		return &op, nil
	default:
		return nil, errors.New("Wrong arguments: updateTopologyOp receives wrong arguments")
	}
}

// Rollback implements the Task interface
func (u *UpdateTopology) Rollback(ctx context.Context) error {
	return nil
}
