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
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/clusterutil"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newDisplayCmd() *cobra.Command {
	var (
		clusterName string
	)
	cmd := &cobra.Command{
		Use:   "display <cluster-name>",
		Short: "Display information of a DM cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName = args[0]
			if err := displayDMMeta(clusterName, &gOpt); err != nil {
				return err
			}
			if err := displayClusterTopology(clusterName, &gOpt); err != nil {
				return err
			}

			metadata, err := meta.DMMetadata(clusterName)
			if err != nil {
				return errors.AddStack(err)
			}
			return clearOutDatedEtcdInfo(clusterName, metadata, gOpt)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only display specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only display specified nodes")
	cmd.Flags().Int64Var(&gOpt.APITimeout, "transfer-timeout", 300, "Timeout in seconds when transferring dm-master leaders")
	return cmd
}

func displayDMMeta(clusterName string, opt *operator.Options) error {
	if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
		return errors.Errorf("cannot display non-exists cluster %s", clusterName)
	}

	clsMeta, err := meta.DMMetadata(clusterName)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold)

	fmt.Printf("DM Cluster: %s\n", cyan.Sprint(clusterName))
	fmt.Printf("DM Version: %s\n", cyan.Sprint(clsMeta.Version))

	return nil
}

func clearOutDatedEtcdInfo(clusterName string, metadata *meta.DMMeta, opt operator.Options) error {
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

	dmMasterClient := api.NewDMMasterClient(topo.GetMasterList(), 10*time.Second, nil)
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

	timeoutOpt := &clusterutil.RetryOption{
		Timeout: time.Second * time.Duration(opt.APITimeout),
		Delay:   time.Second * 2,
	}
	zap.L().Info("Outdated components needed to clear etcd info", zap.Strings("masters", mastersToDelete), zap.Strings("workers", workersToDelete))

	errCh := make(chan error, len(existedMasters)+len(existedWorkers))
	var wg sync.WaitGroup

	for _, master := range mastersToDelete {
		master := master
		wg.Add(1)
		go func() {
			errCh <- dmMasterClient.OfflineMaster(master, timeoutOpt)
			wg.Done()
		}()
	}
	for _, worker := range workersToDelete {
		worker := worker
		wg.Add(1)
		go func() {
			errCh <- dmMasterClient.OfflineWorker(worker, timeoutOpt)
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

func displayClusterTopology(clusterName string, opt *operator.Options) error {
	metadata, err := meta.DMMetadata(clusterName)
	if err != nil {
		return err
	}

	topo := metadata.Topology

	clusterTable := [][]string{
		// Header
		{"ID", "Role", "Host", "Ports", "Status", "Data Dir", "Deploy Dir"},
	}

	ctx := task.NewContext()
	err = ctx.SetSSHKeySet(meta.ClusterPath(clusterName, "ssh", "id_rsa"),
		meta.ClusterPath(clusterName, "ssh", "id_rsa.pub"))
	if err != nil {
		return errors.AddStack(err)
	}

	err = ctx.SetClusterSSH(topo, metadata.User, gOpt.SSHTimeout)
	if err != nil {
		return errors.AddStack(err)
	}

	filterRoles := set.NewStringSet(opt.Roles...)
	filterNodes := set.NewStringSet(opt.Nodes...)
	masterList := topo.GetMasterList()
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, ins := range comp.Instances() {
			// apply role filter
			if len(filterRoles) > 0 && !filterRoles.Exist(ins.Role()) {
				continue
			}
			// apply node filter
			if len(filterNodes) > 0 && !filterNodes.Exist(ins.ID()) {
				continue
			}

			dataDir := "-"
			insDirs := ins.UsedDirs()
			deployDir := insDirs[0]
			if len(insDirs) > 1 {
				dataDir = insDirs[1]
			}

			status := ins.Status(masterList...)
			// Query the service status
			if status == "-" {
				e, found := ctx.GetExecutor(ins.GetHost())
				if found {
					active, _ := operator.GetServiceStatus(e, ins.ServiceName())
					if parts := strings.Split(strings.TrimSpace(active), " "); len(parts) > 2 {
						if parts[1] == "active" {
							status = "Up"
						} else {
							status = parts[1]
						}
					}
				}
			}
			clusterTable = append(clusterTable, []string{
				color.CyanString(ins.ID()),
				ins.Role(),
				ins.GetHost(),
				clusterutil.JoinInt(ins.UsedPorts(), "/"),
				formatInstanceStatus(status),
				dataDir,
				deployDir,
			})

		}
	}

	// Sort by role,host,ports
	sort.Slice(clusterTable[1:], func(i, j int) bool {
		lhs, rhs := clusterTable[i+1], clusterTable[j+1]
		// column: 1 => role, 2 => host, 3 => ports
		for _, col := range []int{1, 2} {
			if lhs[col] != rhs[col] {
				return lhs[col] < rhs[col]
			}
		}
		return lhs[3] < rhs[3]
	})

	cliutil.PrintTable(clusterTable, true)

	return nil
}

func formatInstanceStatus(status string) string {
	lowercaseStatus := strings.ToLower(status)

	startsWith := func(prefixs ...string) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(lowercaseStatus, prefix) {
				return true
			}
		}
		return false
	}

	switch {
	case startsWith("up|l"): // up|l, up|l|ui
		return color.HiGreenString(status)
	case startsWith("up"):
		return color.GreenString(status)
	case startsWith("down", "err"): // down, down|ui
		return color.RedString(status)
	case startsWith("tombstone", "disconnected"), strings.Contains(status, "offline"):
		return color.YellowString(status)
	default:
		return status
	}
}
