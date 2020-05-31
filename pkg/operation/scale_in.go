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

package operator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/pingcap-incubator/tiup-cluster/pkg/api"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
)

// TODO: We can make drainer not async.
var asyncOfflineComps = set.NewStringSet(meta.ComponentPump, meta.ComponentTiKV, meta.ComponentDrainer)

// AsyncNodes return all nodes async destroy or not.
func AsyncNodes(spec meta.Specification, nodes []string, async bool) []string {
	var asyncNodes []string
	var notAsyncNodes []string

	inNodes := func(n string) bool {
		for _, e := range nodes {
			if n == e {
				return true
			}
		}
		return false
	}

	for _, c := range spec.ComponentsByStartOrder() {
		for _, ins := range c.Instances() {
			if !inNodes(ins.ID()) {
				continue
			}

			if asyncOfflineComps.Exist(ins.ComponentName()) {
				asyncNodes = append(asyncNodes, ins.ID())
			} else {
				notAsyncNodes = append(notAsyncNodes, ins.ID())
			}
		}
	}

	if async {
		return asyncNodes
	}

	return notAsyncNodes
}

// ScaleIn scales in the cluster
func ScaleIn(
	getter ExecutorGetter,
	spec meta.Specification,
	options Options,
) error {
	if clusterSpec := spec.GetClusterSpecification(); clusterSpec != nil {
		return ScaleInCluster(getter, clusterSpec, options)
	} else if dmSpec := spec.GetDMSpecification(); dmSpec != nil {
		return ScaleInDMCluster(getter, dmSpec, options)
	}
	return nil
}

// ScaleInCluster scales in the cluster
func ScaleInCluster(
	getter ExecutorGetter,
	spec *meta.ClusterSpecification,
	options Options,
) error {
	// instances by uuid
	instances := map[string]meta.Instance{}

	// make sure all nodeIds exists in topology
	for _, component := range spec.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			instances[instance.ID()] = instance
		}
	}

	// Clean components
	deletedDiff := map[string][]meta.Instance{}
	deletedNodes := set.NewStringSet(options.Nodes...)
	for nodeID := range deletedNodes {
		inst, found := instances[nodeID]
		if !found {
			return errors.Errorf("cannot find node id '%s' in topology", nodeID)
		}
		deletedDiff[inst.ComponentName()] = append(deletedDiff[inst.ComponentName()], inst)
	}

	// Cannot delete all PD servers
	if len(deletedDiff[meta.ComponentPD]) == len(spec.PDServers) {
		return errors.New("cannot delete all PD servers")
	}

	// Cannot delete all TiKV servers
	if len(deletedDiff[meta.ComponentTiKV]) == len(spec.TiKVServers) {
		return errors.New("cannot delete all TiKV servers")
	}

	if options.Force {
		for _, component := range spec.ComponentsByStartOrder() {
			for _, instance := range component.Instances() {
				if !deletedNodes.Exist(instance.ID()) {
					continue
				}
				// just try stop and destroy
				if err := StopComponent(getter, []meta.Instance{instance}); err != nil {
					log.Warnf("failed to stop %s: %v", component.Name(), err)
				}
				if err := DestroyComponent(getter, []meta.Instance{instance}, options.OptTimeout); err != nil {
					log.Warnf("failed to destroy %s: %v", component.Name(), err)
				}

				continue
			}
		}
		return nil
	}

	// TODO if binlog is switch on, cannot delete all pump servers.

	// At least a PD server exists
	var pdClient *api.PDClient
	var pdEndpoint []string
	for _, instance := range (&meta.PDComponent{ClusterSpecification: spec}).Instances() {
		if !deletedNodes.Exist(instance.ID()) {
			pdEndpoint = append(pdEndpoint, addr(instance))
		}
	}

	if len(pdEndpoint) == 0 {
		return errors.New("cannot find available PD instance")
	}

	pdClient = api.NewPDClient(pdEndpoint, 10*time.Second, nil)

	binlogClient, err := api.NewBinlogClient(pdEndpoint, nil /* tls.Config */)
	if err != nil {
		return err
	}

	var tiflashInstances []meta.Instance
	for _, instance := range (&meta.TiFlashComponent{ClusterSpecification: spec}).Instances() {
		if !deletedNodes.Exist(instance.ID()) {
			tiflashInstances = append(tiflashInstances, instance)
		}
	}

	if len(tiflashInstances) > 0 {
		var tikvInstances []meta.Instance
		for _, instance := range (&meta.TiKVComponent{ClusterSpecification: spec}).Instances() {
			if !deletedNodes.Exist(instance.ID()) {
				tikvInstances = append(tikvInstances, instance)
			}
		}

		type replicateConfig struct {
			MaxReplicas int `json:"max-replicas"`
		}

		var config replicateConfig
		bytes, err := pdClient.GetReplicateConfig()
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bytes, &config); err != nil {
			return err
		}

		maxReplicas := config.MaxReplicas

		if len(tikvInstances) < maxReplicas {
			log.Warnf(fmt.Sprintf("TiKV instance number %d will be less than max-replicas setting after scale-in. TiFlash won't be able to receive data from leader before TiKV instance number reach %d", len(tikvInstances), maxReplicas))
		}
	}

	timeoutOpt := &utils.RetryOption{
		Timeout: time.Second * time.Duration(options.APITimeout),
		Delay:   time.Second * 5,
	}

	// Delete member from cluster
	for _, component := range spec.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			if !deletedNodes.Exist(instance.ID()) {
				continue
			}

			switch component.Name() {
			case meta.ComponentTiKV:
				if err := pdClient.DelStore(instance.ID(), timeoutOpt); err != nil {
					return err
				}
			case meta.ComponentTiFlash:
				addr := instance.GetHost() + ":" + strconv.Itoa(instance.(*meta.TiFlashInstance).GetServicePort())
				if err := pdClient.DelStore(addr, timeoutOpt); err != nil {
					return err
				}
			case meta.ComponentPD:
				if err := pdClient.DelPD(instance.(*meta.PDInstance).Name, timeoutOpt); err != nil {
					return err
				}
			case meta.ComponentDrainer:
				addr := instance.GetHost() + ":" + strconv.Itoa(instance.GetPort())
				err := binlogClient.OfflineDrainer(addr, addr)
				if err != nil {
					return errors.AddStack(err)
				}
			case meta.ComponentPump:
				addr := instance.GetHost() + ":" + strconv.Itoa(instance.GetPort())
				err := binlogClient.OfflinePump(addr, addr)
				if err != nil {
					return errors.AddStack(err)
				}
			}

			if !asyncOfflineComps.Exist(instance.ComponentName()) {
				if err := StopComponent(getter, []meta.Instance{instance}); err != nil {
					return errors.Annotatef(err, "failed to stop %s", component.Name())
				}
				if err := DestroyComponent(getter, []meta.Instance{instance}, options.OptTimeout); err != nil {
					return errors.Annotatef(err, "failed to destroy %s", component.Name())
				}
			} else {
				log.Warnf("The component `%s` will be destroyed when display cluster info when it become tombstone, maybe exists in several minutes or hours",
					component.Name())
			}
		}
	}

	for i := 0; i < len(spec.TiKVServers); i++ {
		s := spec.TiKVServers[i]
		id := s.Host + ":" + strconv.Itoa(s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		spec.TiKVServers[i] = s
	}

	for i := 0; i < len(spec.TiFlashServers); i++ {
		s := spec.TiFlashServers[i]
		id := s.Host + ":" + strconv.Itoa(s.TCPPort)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		spec.TiFlashServers[i] = s
	}

	for i := 0; i < len(spec.PumpServers); i++ {
		s := spec.PumpServers[i]
		id := s.Host + ":" + strconv.Itoa(s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		spec.PumpServers[i] = s
	}

	for i := 0; i < len(spec.Drainers); i++ {
		s := spec.Drainers[i]
		id := s.Host + ":" + strconv.Itoa(s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		spec.Drainers[i] = s
	}

	return nil
}

// ScaleInDMCluster scales in the cluster
func ScaleInDMCluster(
	getter ExecutorGetter,
	spec *meta.DMSpecification,
	options Options,
) error {
	// instances by uuid
	instances := map[string]meta.Instance{}

	// make sure all nodeIds exists in topology
	for _, component := range spec.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			instances[instance.ID()] = instance
		}
	}

	// Clean components
	deletedDiff := map[string][]meta.Instance{}
	deletedNodes := set.NewStringSet(options.Nodes...)
	for nodeID := range deletedNodes {
		inst, found := instances[nodeID]
		if !found {
			return errors.Errorf("cannot find node id '%s' in topology", nodeID)
		}
		deletedDiff[inst.ComponentName()] = append(deletedDiff[inst.ComponentName()], inst)
	}

	// Cannot delete all DM Master servers
	if len(deletedDiff[meta.ComponentDMMaster]) == len(spec.Masters) {
		return errors.New("cannot delete all dm-master servers")
	}

	if options.Force {
		for _, component := range spec.ComponentsByStartOrder() {
			for _, instance := range component.Instances() {
				if !deletedNodes.Exist(instance.ID()) {
					continue
				}
				// just try stop and destroy
				if err := StopComponent(getter, []meta.Instance{instance}); err != nil {
					log.Warnf("failed to stop %s: %v", component.Name(), err)
				}
				if err := DestroyComponent(getter, []meta.Instance{instance}, options.OptTimeout); err != nil {
					log.Warnf("failed to destroy %s: %v", component.Name(), err)
				}

				continue
			}
		}
		return nil
	}

	// At least a DMMaster server exists
	var dmMasterClient *api.DMMasterClient
	var dmMasterEndpoint []string
	for _, instance := range (&meta.DMMasterComponent{DMSpecification: spec}).Instances() {
		if !deletedNodes.Exist(instance.ID()) {
			dmMasterEndpoint = append(dmMasterEndpoint, addr(instance))
		}
	}

	if len(dmMasterEndpoint) == 0 {
		return errors.New("cannot find available dm-master instance")
	}

	retryOpt := &utils.RetryOption{
		Timeout: time.Second * time.Duration(options.APITimeout),
		Delay:   time.Second * 2,
	}
	dmMasterClient = api.NewDMMasterClient(dmMasterEndpoint, 10*time.Second, nil)

	// Delete member from cluster
	for _, component := range spec.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			if !deletedNodes.Exist(instance.ID()) {
				continue
			}

			if err := StopComponent(getter, []meta.Instance{instance}); err != nil {
				return errors.Annotatef(err, "failed to stop %s", component.Name())
			}
			if err := DestroyComponent(getter, []meta.Instance{instance}, options.OptTimeout); err != nil {
				return errors.Annotatef(err, "failed to destroy %s", component.Name())
			}

			switch component.Name() {
			case meta.ComponentDMMaster:
				name := instance.(*meta.DMMasterInstance).Name
				err := dmMasterClient.OfflineMaster(name, retryOpt)
				if err != nil {
					return errors.AddStack(err)
				}
			case meta.ComponentDMWorker:
				name := instance.(*meta.DMWorkerInstance).Name
				err := dmMasterClient.OfflineWorker(name, retryOpt)
				if err != nil {
					return errors.AddStack(err)
				}
			}
		}
	}

	return nil
}
