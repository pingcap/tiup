package task

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/set"
	"go.etcd.io/etcd/clientv3"
)

// UpdateTopology is used to maintain the cluster meta information
type UpdateTopology struct {
	cluster        string
	profileDir     string
	metadata       *spec.ClusterMeta
	deletedNodesID []string
}

// String implements the fmt.Stringer interface
func (u *UpdateTopology) String() string {
	return fmt.Sprintf("UpdateTopology: cluster=%s", u.cluster)
}

// Execute implements the Task interface
func (u *UpdateTopology) Execute(ctx *Context) error {
	tlsCfg, err := u.metadata.Topology.TLSConfig(
		filepath.Join(u.profileDir, spec.TLSCertKeyDir),
	)
	if err != nil {
		return err
	}
	client, err := u.metadata.Topology.GetEtcdClient(tlsCfg)
	if err != nil {
		return err
	}
	txn := client.Txn(context.Background())

	topo := u.metadata.Topology

	deleted := set.NewStringSet(u.deletedNodesID...)

	var ops []clientv3.Op
	var instances []spec.Instance

	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.MonitorComponent{Specification: topo}).Instances(), "prometheus")
	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.GrafanaComponent{Specification: topo}).Instances(), "grafana")
	ops, instances = updateInstancesAndOps(ops, instances, deleted, (&spec.AlertManagerComponent{Specification: topo}).Instances(), "alertmanager")

	for _, instance := range (&spec.TiDBComponent{Specification: topo}).Instances() {
		if deleted.Exist(instance.ID()) {
			ops = append(ops, clientv3.OpDelete(fmt.Sprintf("/topology/tidb/%s:%d", instance.GetHost(), instance.GetPort()), clientv3.WithPrefix()))
		}
	}

	for _, ins := range instances {
		op, err := updateTopologyOp(ins)
		if err != nil {
			return err
		}
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

// componentTopology update receives alertmanager, prometheus and grafana instance list, if the list has
//  no member or all deleted, it will add a `OpDelete` in ops, otherwise it will push an operation to destInstances.
func updateInstancesAndOps(ops []clientv3.Op, destInstances []spec.Instance, deleted set.StringSet, instances []spec.Instance, componentName string) ([]clientv3.Op, []spec.Instance) {
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
		destInstances = append(destInstances, currentInstances...)
	}
	return ops, destInstances
}

// updateTopologyOp receive a  alertmanager, prometheus or grafana instance, and return an operation
//  for update it's topology.
func updateTopologyOp(instance spec.Instance) (*clientv3.Op, error) {
	switch instance.ComponentName() {
	case spec.ComponentAlertManager, spec.ComponentPrometheus, spec.ComponentGrafana:
		topology := componentTopology{
			IP:         instance.GetHost(),
			Port:       instance.GetPort(),
			DeployPath: instance.DeployDir(),
		}
		data, err := json.Marshal(topology)
		if err != nil {
			return nil, err
		}
		op := clientv3.OpPut("/topology/"+instance.ComponentName(), string(data))
		return &op, nil
	default:
		return nil, errors.New("Wrong arguments: updateTopologyOp receive wrong arguments")
	}
}

// Rollback implements the Task interface
func (u *UpdateTopology) Rollback(ctx *Context) error {
	return nil
}
