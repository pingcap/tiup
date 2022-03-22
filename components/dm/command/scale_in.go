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

package command

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/pingcap/errors"
	dm "github.com/pingcap/tiup/components/dm/spec"
	dmtask "github.com/pingcap/tiup/components/dm/task"
	"github.com/pingcap/tiup/pkg/cluster/api"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/spf13/cobra"
)

func newScaleInCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale-in <cluster-name>",
		Short: "Scale in a DM cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]

			scale := func(b *task.Builder, imetadata spec.Metadata, tlsCfg *tls.Config) {
				metadata := imetadata.(*dm.Metadata)
				b.Func(
					fmt.Sprintf("ScaleInCluster: options=%+v", gOpt),
					func(ctx context.Context) error {
						return ScaleInDMCluster(ctx, metadata.Topology, gOpt, tlsCfg)
					},
				).Serial(dmtask.NewUpdateDMMeta(clusterName, metadata, gOpt.Nodes))
			}

			return cm.ScaleIn(clusterName, skipConfirm, gOpt, scale)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Specify the nodes (required)")
	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force just try stop and destroy instance before removing the instance from topo")

	_ = cmd.MarkFlagRequired("node")

	return cmd
}

// ScaleInDMCluster scale in dm cluster.
func ScaleInDMCluster(
	ctx context.Context,
	topo *dm.Specification,
	options operator.Options,
	tlsCfg *tls.Config,
) error {
	// instances by uuid
	instances := map[string]dm.Instance{}
	instCount := map[string]int{}

	// make sure all nodeIds exists in topology
	for _, component := range topo.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			instances[instance.ID()] = instance
			instCount[instance.GetHost()]++
		}
	}

	// Clean components
	deletedDiff := map[string][]dm.Instance{}
	deletedNodes := set.NewStringSet(options.Nodes...)
	for nodeID := range deletedNodes {
		inst, found := instances[nodeID]
		if !found {
			return errors.Errorf("cannot find node id '%s' in topology", nodeID)
		}
		deletedDiff[inst.ComponentName()] = append(deletedDiff[inst.ComponentName()], inst)
	}

	// Cannot delete all DM DMMaster servers
	if len(deletedDiff[dm.ComponentDMMaster]) == len(topo.Masters) {
		return errors.New("cannot delete all dm-master servers")
	}

	if options.Force {
		for _, component := range topo.ComponentsByStartOrder() {
			for _, instance := range component.Instances() {
				if !deletedNodes.Exist(instance.ID()) {
					continue
				}
				instCount[instance.GetHost()]--
				if err := operator.StopAndDestroyInstance(ctx, topo, instance, options, instCount[instance.GetHost()] == 0); err != nil {
					log.Warnf("failed to stop/destroy %s: %v", component.Name(), err)
				}
			}
		}
		return nil
	}

	// At least a DMMaster server exists
	var dmMasterClient *api.DMMasterClient
	var dmMasterEndpoint []string
	for _, instance := range (&dm.DMMasterComponent{Topology: topo}).Instances() {
		if !deletedNodes.Exist(instance.ID()) {
			dmMasterEndpoint = append(dmMasterEndpoint, operator.Addr(instance))
		}
	}

	if len(dmMasterEndpoint) == 0 {
		return errors.New("cannot find available dm-master instance")
	}

	dmMasterClient = api.NewDMMasterClient(dmMasterEndpoint, 10*time.Second, tlsCfg)

	noAgentHosts := set.NewStringSet()
	topo.IterInstance(func(inst dm.Instance) {
		if inst.IgnoreMonitorAgent() {
			noAgentHosts.Insert(inst.GetHost())
		}
	})

	// Delete member from cluster
	for _, component := range topo.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			if !deletedNodes.Exist(instance.ID()) {
				continue
			}

			if err := operator.StopComponent(
				ctx,
				topo,
				[]dm.Instance{instance},
				noAgentHosts,
				options.OptTimeout,
				false,         /* evictLeader */
				&tls.Config{}, /* not used as evictLeader is false */
			); err != nil {
				return errors.Annotatef(err, "failed to stop %s", component.Name())
			}

			switch component.Name() {
			case dm.ComponentDMMaster:
				name := instance.(*dm.MasterInstance).Name
				err := dmMasterClient.OfflineMaster(name, nil)
				if err != nil {
					return err
				}
			case dm.ComponentDMWorker:
				name := instance.(*dm.WorkerInstance).Name
				err := dmMasterClient.OfflineWorker(name, nil)
				if err != nil {
					return err
				}
			}

			if err := operator.DestroyComponent(ctx, []dm.Instance{instance}, topo, options); err != nil {
				return errors.Annotatef(err, "failed to destroy %s", component.Name())
			}

			instCount[instance.GetHost()]--
			if instCount[instance.GetHost()] == 0 {
				if err := operator.DeletePublicKey(ctx, instance.GetHost()); err != nil {
					return errors.Annotatef(err, "failed to delete public key")
				}
			}
		}
	}

	return nil
}
