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

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
)

// TODO: We can make drainer not async.
var asyncOfflineComps = set.NewStringSet(spec.ComponentPump, spec.ComponentTiKV, spec.ComponentTiFlash, spec.ComponentDrainer)

// AsyncNodes return all nodes async destroy or not.
func AsyncNodes(spec *spec.Specification, nodes []string, async bool) []string {
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
	cluster *spec.Specification,
	options Options,
) error {
	return ScaleInCluster(getter, cluster, options)
}

// ScaleInCluster scales in the cluster
func ScaleInCluster(
	getter ExecutorGetter,
	cluster *spec.Specification,
	options Options,
) error {
	// instances by uuid
	instances := map[string]spec.Instance{}

	// make sure all nodeIds exists in topology
	for _, component := range cluster.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			instances[instance.ID()] = instance
		}
	}

	// Clean components
	deletedDiff := map[string][]spec.Instance{}
	deletedNodes := set.NewStringSet(options.Nodes...)
	for nodeID := range deletedNodes {
		inst, found := instances[nodeID]
		if !found {
			return errors.Errorf("cannot find node id '%s' in topology", nodeID)
		}
		deletedDiff[inst.ComponentName()] = append(deletedDiff[inst.ComponentName()], inst)
	}

	// Cannot delete all PD servers
	if len(deletedDiff[spec.ComponentPD]) == len(cluster.PDServers) {
		return errors.New("cannot delete all PD servers")
	}

	// Cannot delete all TiKV servers
	if len(deletedDiff[spec.ComponentTiKV]) == len(cluster.TiKVServers) {
		return errors.New("cannot delete all TiKV servers")
	}

	var pdEndpoint []string
	for _, instance := range (&spec.PDComponent{Specification: cluster}).Instances() {
		if !deletedNodes.Exist(instance.ID()) {
			pdEndpoint = append(pdEndpoint, Addr(instance))
		}
	}

	if options.Force {
		for _, component := range cluster.ComponentsByStartOrder() {
			for _, instance := range component.Instances() {
				if !deletedNodes.Exist(instance.ID()) {
					continue
				}

				pdClient := api.NewPDClient(pdEndpoint, 10*time.Second, nil)
				binlogClient, _ := api.NewBinlogClient(pdEndpoint, nil /* tls.Config */)

				if component.Name() != spec.ComponentPump && component.Name() != spec.ComponentDrainer {
					if err := deleteMember(component, instance, pdClient, binlogClient, options.APITimeout); err != nil {
						log.Warnf("failed to delete %s: %v", component.Name(), err)
					}
				}

				// just try stop and destroy
				if err := StopComponent(getter, []spec.Instance{instance}, options.OptTimeout); err != nil {
					log.Warnf("failed to stop %s: %v", component.Name(), err)
				}
				if err := DestroyComponent(getter, []spec.Instance{instance}, cluster, options); err != nil {
					log.Warnf("failed to destroy %s: %v", component.Name(), err)
				}

				// directly update pump&drainer 's state as offline in etcd.
				if binlogClient != nil {
					id := instance.ID()
					if component.Name() == spec.ComponentPump {
						if err := binlogClient.UpdatePumpState(id, "offline"); err != nil {
							log.Warnf("failed to update %s state as offline: %v", component.Name(), err)
						}
					} else if component.Name() == spec.ComponentDrainer {
						if err := binlogClient.UpdateDrainerState(id, "offline"); err != nil {
							log.Warnf("failed to update %s state as offline: %v", component.Name(), err)
						}
					}
				}

				continue
			}
		}
		return nil
	}

	// TODO if binlog is switch on, cannot delete all pump servers.

	// At least a PD server exists
	var pdClient *api.PDClient

	if len(pdEndpoint) == 0 {
		return errors.New("cannot find available PD instance")
	}

	pdClient = api.NewPDClient(pdEndpoint, 10*time.Second, nil)

	binlogClient, err := api.NewBinlogClient(pdEndpoint, nil /* tls.Config */)
	if err != nil {
		return err
	}

	var tiflashInstances []spec.Instance
	for _, instance := range (&spec.TiFlashComponent{Specification: cluster}).Instances() {
		if !deletedNodes.Exist(instance.ID()) {
			tiflashInstances = append(tiflashInstances, instance)
		}
	}

	if len(tiflashInstances) > 0 {
		var tikvInstances []spec.Instance
		for _, instance := range (&spec.TiKVComponent{Specification: cluster}).Instances() {
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

	// Delete member from cluster
	for _, component := range cluster.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			if !deletedNodes.Exist(instance.ID()) {
				continue
			}

			err := deleteMember(component, instance, pdClient, binlogClient, options.APITimeout)
			if err != nil {
				return errors.Trace(err)
			}

			if !asyncOfflineComps.Exist(instance.ComponentName()) {
				if err := StopComponent(getter, []spec.Instance{instance}, options.OptTimeout); err != nil {
					return errors.Annotatef(err, "failed to stop %s", component.Name())
				}
				if err := DestroyComponent(getter, []spec.Instance{instance}, cluster, options); err != nil {
					return errors.Annotatef(err, "failed to destroy %s", component.Name())
				}
			} else {
				log.Warnf(color.YellowString("The component `%s` will be destroyed when display cluster info when it become tombstone, maybe exists in several minutes or hours",
					component.Name()))
			}
		}
	}

	for i := 0; i < len(cluster.TiKVServers); i++ {
		s := cluster.TiKVServers[i]
		id := s.Host + ":" + strconv.Itoa(s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.TiKVServers[i] = s
	}

	for i := 0; i < len(cluster.TiFlashServers); i++ {
		s := cluster.TiFlashServers[i]
		id := s.Host + ":" + strconv.Itoa(s.TCPPort)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.TiFlashServers[i] = s
	}

	for i := 0; i < len(cluster.PumpServers); i++ {
		s := cluster.PumpServers[i]
		id := s.Host + ":" + strconv.Itoa(s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.PumpServers[i] = s
	}

	for i := 0; i < len(cluster.Drainers); i++ {
		s := cluster.Drainers[i]
		id := s.Host + ":" + strconv.Itoa(s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.Drainers[i] = s
	}

	return nil
}

func deleteMember(
	component spec.Component,
	instance spec.Instance,
	pdClient *api.PDClient,
	binlogClient *api.BinlogClient,
	timeoutSecond int64,
) error {
	timeoutOpt := &utils.RetryOption{
		Timeout: time.Second * time.Duration(timeoutSecond),
		Delay:   time.Second * 5,
	}

	switch component.Name() {
	case spec.ComponentTiKV:
		if err := pdClient.DelStore(instance.ID(), timeoutOpt); err != nil {
			return err
		}
	case spec.ComponentTiFlash:
		addr := instance.GetHost() + ":" + strconv.Itoa(instance.(*spec.TiFlashInstance).GetServicePort())
		if err := pdClient.DelStore(addr, timeoutOpt); err != nil {
			return err
		}
	case spec.ComponentPD:
		if err := pdClient.DelPD(instance.(*spec.PDInstance).Name, timeoutOpt); err != nil {
			return err
		}
	case spec.ComponentDrainer:
		addr := instance.GetHost() + ":" + strconv.Itoa(instance.GetPort())
		err := binlogClient.OfflineDrainer(addr, addr)
		if err != nil {
			return errors.AddStack(err)
		}
	case spec.ComponentPump:
		addr := instance.GetHost() + ":" + strconv.Itoa(instance.GetPort())
		err := binlogClient.OfflinePump(addr, addr)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	return nil
}
