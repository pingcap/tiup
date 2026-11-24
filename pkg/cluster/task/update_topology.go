package task

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
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

	// Whether an empty surviving Grafana list may keep the discovery entry
	// depends on what the entry currently holds, which only the cluster
	// itself knows: the metadata's detached records vouch for an address,
	// not for whatever the entry happens to contain. The read sits outside
	// the transaction below; the entry is written only by TiUP, and
	// refreshes of one cluster are not expected to interleave.
	grafanaEntry, err := readTopologyEntry(ctx, client, spec.ComponentGrafana)
	if err != nil {
		return err
	}

	var ops []clientv3.Op
	var instances []spec.Instance

	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.MonitorComponent{Topology: topo}).Instances(), "prometheus", nil, nil)
	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.GrafanaComponent{Topology: topo}).Instances(), "grafana", topo.DetachedGrafanas, grafanaEntry)
	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.AlertManagerComponent{Topology: topo}).Instances(), "alertmanager", nil, nil)

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

	_, err = txn.Then(ops...).Commit()
	return err
}

// componentTopology represent the topology info for alertmanager, prometheus and grafana.
type componentTopology struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	DeployPath string `json:"deploy_path"`
}

// updateInstancesAndOps receives alertmanager, prometheus and grafana instance list, if the list has
//
//	no member or all deleted, it will add a `OpDelete` in ops, otherwise it will push all current not deleted instances into instance list.
//
// For Grafana, an empty surviving list keeps the discovery entry only while
// the entry's current address, grafanaEntry, belongs to an instance TiUP
// handed over (a detached record). The records vouch for their addresses
// alone: refreshes rewrite the entry to the surviving managed instance, and
// once that one is scaled in too, the entry holds a dead address no record
// vouches for — deleting it lets every later refresh converge the stale
// entry away, as before detach-grafana existed. Without any record the same
// holds for a scale-in whose PD cleanup failed midway. A missing entry
// stays missing; nothing writes one back. The check runs on the survivors,
// after the deleted filter, so a scale-in of the last managed Grafana also
// reaches it and cannot delete an entry preserved for a detached one.
func updateInstancesAndOps(ops []clientv3.Op, ins []spec.Instance, deleted set.StringSet, instances []spec.Instance, componentName string, detachedGrafanas []*spec.DetachedGrafana, grafanaEntry *componentTopology) ([]clientv3.Op, []spec.Instance) {
	var currentInstances []spec.Instance
	for _, instance := range instances {
		if deleted.Exist(instance.ID()) {
			continue
		}
		currentInstances = append(currentInstances, instance)
	}

	if componentName == spec.ComponentGrafana && len(currentInstances) == 0 && len(detachedGrafanas) > 0 {
		if grafanaEntry == nil {
			return ops, ins
		}
		for _, detached := range detachedGrafanas {
			if detached.Matches(grafanaEntry.IP, grafanaEntry.Port) {
				return ops, ins
			}
		}
	}

	if len(currentInstances) == 0 {
		ops = append(ops, clientv3.OpDelete("/topology/"+componentName))
	} else {
		ins = append(ins, currentInstances...)
	}
	return ops, ins
}

// readTopologyEntry returns the current value of a monitoring component's
// discovery entry, or nil when the entry does not exist.
func readTopologyEntry(ctx context.Context, client *clientv3.Client, component string) (*componentTopology, error) {
	resp, err := client.Get(ctx, "/topology/"+component)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var entry componentTopology
	if err := json.Unmarshal(resp.Kvs[0].Value, &entry); err != nil {
		return nil, errors.Annotatef(err, "unmarshal discovery entry /topology/%s", component)
	}
	return &entry, nil
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
