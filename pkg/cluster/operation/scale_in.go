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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/sync/errgroup"
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
	ctx context.Context,
	cluster *spec.Specification,
	options Options,
	tlsCfg *tls.Config,
) error {
	return ScaleInCluster(ctx, cluster, options, tlsCfg)
}

// ScaleInCluster scales in the cluster
//
//revive:disable
func ScaleInCluster(
	ctx context.Context,
	cluster *spec.Specification,
	options Options,
	tlsCfg *tls.Config,
) error {
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	// instances by uuid
	instances := map[string]spec.Instance{}
	instCount := map[string]int{}

	// make sure all nodeIds exists in topology
	for _, component := range cluster.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			instances[instance.ID()] = instance
			instCount[instance.GetManageHost()]++
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

	skipTopoCheck := false
	if v := os.Getenv(EnvNameSkipScaleInTopoCheck); v != "" { // any value except empty will work as "true"
		skipTopoCheck = true
	}

	if skipTopoCheck {
		logger.Warnf("%s is set, topology checks ignored, the cluster might be broken after the operations!", EnvNameSkipScaleInTopoCheck)
		if ok, input := tui.PromptForConfirmYes("Are you sure to continue? [y/N]"); !ok {
			return errors.Errorf("user aborted with '%s'", input)
		}
	} else {
		// Cannot delete all PD servers
		if len(deletedDiff[spec.ComponentPD]) == len(cluster.PDServers) {
			return errors.New("cannot delete all PD servers")
		}

		// Cannot delete all TiKV servers
		if len(deletedDiff[spec.ComponentTiKV]) == len(cluster.TiKVServers) {
			return errors.New("cannot delete all TiKV servers")
		}

		// Cannot delete TiSpark master server if there's any TiSpark worker remains
		if len(deletedDiff[spec.ComponentTiSpark]) > 0 {
			var cntDiffTiSparkMaster int
			var cntDiffTiSparkWorker int
			for _, inst := range deletedDiff[spec.ComponentTiSpark] {
				switch inst.Role() {
				case spec.RoleTiSparkMaster:
					cntDiffTiSparkMaster++
				case spec.RoleTiSparkWorker:
					cntDiffTiSparkWorker++
				}
			}
			if cntDiffTiSparkMaster == len(cluster.TiSparkMasters) &&
				cntDiffTiSparkWorker < len(cluster.TiSparkWorkers) {
				return errors.New("cannot delete tispark master when there are workers left")
			}
		}
	}

	var pdEndpoints []string
	forcePDEndpoints := os.Getenv(EnvNamePDEndpointOverwrite) // custom set PD endpoint list

	if forcePDEndpoints != "" {
		pdEndpoints = strings.Split(forcePDEndpoints, ",")
		logger.Warnf("%s is set, using %s as PD endpoints", EnvNamePDEndpointOverwrite, pdEndpoints)
	} else {
		for _, instance := range (&spec.PDComponent{Topology: cluster}).Instances() {
			if !deletedNodes.Exist(instance.ID()) {
				pdEndpoints = append(pdEndpoints, Addr(instance))
			}
		}
	}

	// At least a PD server exists
	if len(pdEndpoints) == 0 {
		return errors.New("cannot find available PD instance")
	}

	pdClient := api.NewPDClient(ctx, pdEndpoints, 10*time.Second, tlsCfg)

	tcpProxy := proxy.GetTCPProxy()
	if tcpProxy != nil {
		closeC := tcpProxy.Run(pdEndpoints)
		defer tcpProxy.Close(closeC)
		pdEndpoints = tcpProxy.GetEndpoints()
	}
	binlogClient, err := api.NewBinlogClient(pdEndpoints, 5*time.Second, tlsCfg)
	if err != nil {
		return err
	}

	if options.Force {
		for _, component := range cluster.ComponentsByStartOrder() {
			for _, instance := range component.Instances() {
				if !deletedNodes.Exist(instance.ID()) {
					continue
				}
				compName := component.Name()

				if compName != spec.ComponentPump && compName != spec.ComponentDrainer {
					if err := deleteMember(ctx, component, instance, pdClient, binlogClient, options.APITimeout); err != nil {
						logger.Warnf("failed to delete %s: %v", compName, err)
					}
				}

				instCount[instance.GetManageHost()]--
				if err := StopAndDestroyInstance(ctx, cluster, instance, options, true, instCount[instance.GetManageHost()] == 0, tlsCfg); err != nil {
					logger.Warnf("failed to stop/destroy %s: %v", compName, err)
				}

				// directly update pump&drainer 's state as offline in etcd.
				if binlogClient != nil {
					id := instance.ID()
					if compName == spec.ComponentPump {
						if err := binlogClient.UpdatePumpState(ctx, id, "offline"); err != nil {
							logger.Warnf("failed to update %s state as offline: %v", compName, err)
						}
					} else if compName == spec.ComponentDrainer {
						if err := binlogClient.UpdateDrainerState(ctx, id, "offline"); err != nil {
							logger.Warnf("failed to update %s state as offline: %v", compName, err)
						}
					}
				}
			}
		}
		return nil
	}

	// TODO if binlog is switch on, cannot delete all pump servers.

	var tiflashInstances []spec.Instance
	for _, instance := range (&spec.TiFlashComponent{Topology: cluster}).Instances() {
		if !deletedNodes.Exist(instance.ID()) {
			tiflashInstances = append(tiflashInstances, instance)
		}
	}

	if len(tiflashInstances) > 0 {
		var tikvInstances []spec.Instance
		for _, instance := range (&spec.TiKVComponent{Topology: cluster}).Instances() {
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
			logger.Warnf(fmt.Sprintf("TiKV instance number %d will be less than max-replicas setting after scale-in. TiFlash won't be able to receive data from leader before TiKV instance number reach %d", len(tikvInstances), maxReplicas))
		}
	}

	cdcInstances := make([]spec.Instance, 0)
	// Delete member from cluster
	for _, component := range cluster.ComponentsByStartOrder() {
		deferInstances := make([]spec.Instance, 0)
		for _, instance := range component.Instances() {
			if !deletedNodes.Exist(instance.ID()) {
				continue
			}

			// skip cdc at the moment, handle them separately.
			if component.Role() == spec.ComponentCDC {
				cdcInstances = append(cdcInstances, instance)
				continue
			}

			if component.Role() == spec.ComponentPD {
				// defer PD leader to be scale-in after others
				isLeader, err := instance.(*spec.PDInstance).IsLeader(ctx, cluster, int(options.APITimeout), tlsCfg)
				if err != nil {
					logger.Warnf("cannot found pd leader, ignore: %s", err)
					return err
				}
				if isLeader {
					deferInstances = append(deferInstances, instance)
					logger.Debugf("Deferred scale-in of PD leader %s", instance.ID())
					continue
				}
			}

			err := deleteMember(ctx, component, instance, pdClient, binlogClient, options.APITimeout)
			if err != nil {
				return errors.Trace(err)
			}

			if !asyncOfflineComps.Exist(instance.ComponentName()) {
				instCount[instance.GetManageHost()]--
				if err := StopAndDestroyInstance(ctx, cluster, instance, options, false, instCount[instance.GetManageHost()] == 0, tlsCfg); err != nil {
					return err
				}
			} else {
				logger.Warnf(color.YellowString("The component `%s` will become tombstone, maybe exists in several minutes or hours, after that you can use the prune command to clean it",
					component.Name()))
			}
		}

		// process deferred instances
		for _, instance := range deferInstances {
			// actually, it must be the pd leader at the moment, so the `PreRestart` always triggered.
			rollingInstance, ok := instance.(spec.RollingUpdateInstance)
			if ok {
				if err := rollingInstance.PreRestart(ctx, cluster, int(options.APITimeout), tlsCfg); err != nil {
					return errors.Trace(err)
				}
			}

			err := deleteMember(ctx, component, instance, pdClient, binlogClient, options.APITimeout)
			if err != nil {
				return errors.Trace(err)
			}

			if !asyncOfflineComps.Exist(instance.ComponentName()) {
				instCount[instance.GetManageHost()]--
				if err := StopAndDestroyInstance(ctx, cluster, instance, options, false, instCount[instance.GetManageHost()] == 0, tlsCfg); err != nil {
					return err
				}
			} else {
				logger.Warnf(color.YellowString("The component `%s` will become tombstone, maybe exists in several minutes or hours, after that you can use the prune command to clean it",
					component.Name()))
			}
		}
	}

	if len(cdcInstances) != 0 {
		err := scaleInCDC(ctx, cluster, cdcInstances, tlsCfg, options, instCount)
		if err != nil {
			return errors.Trace(err)
		}
	}

	for i := 0; i < len(cluster.TiKVServers); i++ {
		s := cluster.TiKVServers[i]
		id := utils.JoinHostPort(s.Host, s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.TiKVServers[i] = s
	}

	for i := 0; i < len(cluster.TiFlashServers); i++ {
		s := cluster.TiFlashServers[i]
		id := utils.JoinHostPort(s.Host, s.TCPPort)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.TiFlashServers[i] = s
	}

	for i := 0; i < len(cluster.PumpServers); i++ {
		s := cluster.PumpServers[i]
		id := utils.JoinHostPort(s.Host, s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.PumpServers[i] = s
	}

	for i := 0; i < len(cluster.Drainers); i++ {
		s := cluster.Drainers[i]
		id := utils.JoinHostPort(s.Host, s.Port)
		if !deletedNodes.Exist(id) {
			continue
		}
		s.Offline = true
		cluster.Drainers[i] = s
	}

	return nil
}

func deleteMember(
	ctx context.Context,
	component spec.Component,
	instance spec.Instance,
	pdClient *api.PDClient,
	binlogClient *api.BinlogClient,
	timeoutSecond uint64,
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
		addr := utils.JoinHostPort(instance.GetHost(), instance.(*spec.TiFlashInstance).GetServicePort())
		if err := pdClient.DelStore(addr, timeoutOpt); err != nil {
			return err
		}
	case spec.ComponentPD:
		if err := pdClient.DelPD(instance.(*spec.PDInstance).Name, timeoutOpt); err != nil {
			return err
		}
	case spec.ComponentDrainer:
		addr := utils.JoinHostPort(instance.GetHost(), instance.GetPort())
		err := binlogClient.OfflineDrainer(ctx, addr)
		if err != nil {
			return err
		}
	case spec.ComponentPump:
		addr := utils.JoinHostPort(instance.GetHost(), instance.GetPort())
		err := binlogClient.OfflinePump(ctx, addr)
		if err != nil {
			return err
		}
	}

	return nil
}

func scaleInCDC(
	ctx context.Context,
	cluster *spec.Specification,
	instances []spec.Instance,
	tlsCfg *tls.Config,
	options Options,
	instCount map[string]int,
) error {
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)

	// if all cdc instances are selected, just stop all instances by force
	if len(instances) == len(cluster.CDCServers) {
		g, _ := errgroup.WithContext(ctx)
		for _, ins := range instances {
			ins := ins
			instCount[ins.GetManageHost()]++
			destroyNode := instCount[ins.GetManageHost()] == 0
			g.Go(func() error {
				return StopAndDestroyInstance(ctx, cluster, ins, options, true, destroyNode, tlsCfg)
			})
		}
		return g.Wait()
	}

	deferInstances := make([]spec.Instance, 0, 1)
	for _, instance := range instances {
		address := instance.(*spec.CDCInstance).GetAddr()
		client := api.NewCDCOpenAPIClient(ctx, []string{utils.JoinHostPort(instance.GetManageHost(), instance.GetPort())}, 5*time.Second, tlsCfg)

		capture, err := client.GetCaptureByAddr(address)
		if err != nil {
			// this may be caused by that the instance is not running, or the specified version of cdc does not support open api
			logger.Debugf("scale-in cdc, get capture by address failed, stop the instance by force, "+
				"addr: %s, err: %+v", address, err)
			instCount[instance.GetManageHost()]--
			if err := StopAndDestroyInstance(ctx, cluster, instance, options, true, instCount[instance.GetManageHost()] == 0, tlsCfg); err != nil {
				return err
			}
			continue
		}

		if capture.IsOwner {
			deferInstances = append(deferInstances, instance)
			logger.Debugf("Deferred scale-in the TiCDC owner %s", instance.ID())
			continue
		}

		instCount[instance.GetManageHost()]--
		if err := StopAndDestroyInstance(ctx, cluster, instance, options, false, instCount[instance.GetManageHost()] == 0, tlsCfg); err != nil {
			return err
		}
	}

	for _, instance := range deferInstances {
		instCount[instance.GetManageHost()]--
		if err := StopAndDestroyInstance(ctx, cluster, instance, options, false, instCount[instance.GetManageHost()] == 0, tlsCfg); err != nil {
			return err
		}
	}

	return nil
}
