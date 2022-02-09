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
	"sync"
	"time"

	"github.com/pingcap/tiup/components/dm/spec"
	"github.com/pingcap/tiup/pkg/cluster/api"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	tidbspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune <cluster-name>",
		Short: "Clear etcd info ",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]

			metadata := new(spec.Metadata)
			err := dmspec.Metadata(clusterName, metadata)
			if err != nil {
				return err
			}

			return clearOutDatedEtcdInfo(clusterName, metadata, gOpt)
		},
	}

	return cmd
}

func clearOutDatedEtcdInfo(clusterName string, metadata *spec.Metadata, opt operator.Options) error {
	topo := metadata.Topology

	existedMasters := make(map[string]struct{})
	existedWorkers := make(map[string]struct{})
	mastersToDelete := make([]string, 0)
	workersToDelete := make([]string, 0)

	for _, masterSpec := range topo.Masters {
		existedMasters[masterSpec.Name] = struct{}{}
	}
	for _, workerSpec := range topo.Workers {
		existedWorkers[workerSpec.Name] = struct{}{}
	}

	tlsCfg, err := topo.TLSConfig(dmspec.Path(clusterName, tidbspec.TLSCertKeyDir))
	if err != nil {
		return err
	}
	dmMasterClient := api.NewDMMasterClient(topo.GetMasterList(), 10*time.Second, tlsCfg)
	registeredMasters, registeredWorkers, err := dmMasterClient.GetRegisteredMembers()
	if err != nil {
		return err
	}

	for _, master := range registeredMasters {
		if _, ok := existedMasters[master]; !ok {
			mastersToDelete = append(mastersToDelete, master)
		}
	}
	for _, worker := range registeredWorkers {
		if _, ok := existedWorkers[worker]; !ok {
			workersToDelete = append(workersToDelete, worker)
		}
	}

	zap.L().Info("Outdated components needed to clear etcd info", zap.Strings("masters", mastersToDelete), zap.Strings("workers", workersToDelete))

	errCh := make(chan error, len(existedMasters)+len(existedWorkers))
	var wg sync.WaitGroup

	for _, master := range mastersToDelete {
		master := master
		wg.Add(1)
		go func() {
			errCh <- dmMasterClient.OfflineMaster(master, nil)
			wg.Done()
		}()
	}
	for _, worker := range workersToDelete {
		worker := worker
		wg.Add(1)
		go func() {
			errCh <- dmMasterClient.OfflineWorker(worker, nil)
			wg.Done()
		}()
	}

	wg.Wait()
	if len(errCh) == 0 {
		return nil
	}

	// return any one error
	return <-errCh
}
