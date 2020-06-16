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
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newDisplayCmd() *cobra.Command {
	var (
		clusterName       string
		showDashboardOnly bool
	)
	cmd := &cobra.Command{
		Use:   "display <cluster-name>",
		Short: "Display information of a TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName = args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return perrs.Errorf("Cluster %s not found", clusterName)
			}

			if showDashboardOnly {
				return displayDashboardInfo(clusterName)
			}

			if err := displayClusterMeta(clusterName, &gOpt); err != nil {
				return err
			}
			if err := displayClusterTopology(clusterName, &gOpt); err != nil {
				return err
			}

			metadata, err := meta.ClusterMetadata(clusterName)
			if err != nil && !errors.Is(perrs.Cause(err), &meta.ValidateErr{}) {
				return perrs.AddStack(err)
			}
			return destroyTombstoneIfNeed(clusterName, metadata, gOpt)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only display specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only display specified nodes")
	cmd.Flags().BoolVar(&showDashboardOnly, "dashboard", false, "Only display TiDB Dashboard information")

	return cmd
}

func displayDashboardInfo(clusterName string) error {
	metadata, err := meta.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), &meta.ValidateErr{}) {
		return err
	}

	pdEndpoints := make([]string, 0)
	for _, pd := range metadata.Topology.PDServers {
		pdEndpoints = append(pdEndpoints, fmt.Sprintf("%s:%d", pd.Host, pd.ClientPort))
	}

	pdAPI := api.NewPDClient(pdEndpoints, 2*time.Second, nil)
	dashboardAddr, err := pdAPI.GetDashboardAddress()
	if err != nil {
		return fmt.Errorf("failed to retrieve TiDB Dashboard instance from PD: %s", err)
	}
	if dashboardAddr == "auto" {
		return fmt.Errorf("TiDB Dashboard is not initialized, please start PD and try again")
	} else if dashboardAddr == "none" {
		return fmt.Errorf("TiDB Dashboard is disabled")
	}

	u, err := url.Parse(dashboardAddr)
	if err != nil {
		return fmt.Errorf("unknown TiDB Dashboard PD instance: %s", dashboardAddr)
	}

	u.Path = "/dashboard/"
	fmt.Println(u.String())

	return nil
}

func displayClusterMeta(clusterName string, opt *operator.Options) error {
	clsMeta, err := meta.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), &meta.ValidateErr{}) {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold)

	fmt.Printf("TiDB Cluster: %s\n", cyan.Sprint(clusterName))
	fmt.Printf("TiDB Version: %s\n", cyan.Sprint(clsMeta.Version))

	return nil
}

func destroyTombstoneIfNeed(clusterName string, metadata *meta.ClusterMeta, opt operator.Options) error {
	topo := metadata.Topology

	if !operator.NeedCheckTomebsome(topo) {
		return nil
	}

	ctx := task.NewContext()
	err := ctx.SetSSHKeySet(meta.ClusterPath(clusterName, "ssh", "id_rsa"),
		meta.ClusterPath(clusterName, "ssh", "id_rsa.pub"))
	if err != nil {
		return perrs.AddStack(err)
	}

	err = ctx.SetClusterSSH(topo, metadata.User, gOpt.SSHTimeout)
	if err != nil {
		return perrs.AddStack(err)
	}

	nodes, err := operator.DestroyTombstone(ctx, topo, true /* returnNodesOnly */, opt)
	if err != nil {
		return perrs.AddStack(err)
	}

	if len(nodes) == 0 {
		return nil
	}

	log.Infof("Start destroy Tombstone nodes: %v ...", nodes)

	_, err = operator.DestroyTombstone(ctx, topo, false /* returnNodesOnly */, opt)
	if err != nil {
		return perrs.AddStack(err)
	}

	log.Infof("Destroy success")

	return meta.SaveClusterMeta(clusterName, metadata)
}

func displayClusterTopology(clusterName string, opt *operator.Options) error {
	metadata, err := meta.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), &meta.ValidateErr{}) {
		return err
	}

	topo := metadata.Topology

	clusterTable := [][]string{
		// Header
		{"ID", "Role", "Host", "Ports", "OS/Arch", "Status", "Data Dir", "Deploy Dir"},
	}

	ctx := task.NewContext()
	err = ctx.SetSSHKeySet(meta.ClusterPath(clusterName, "ssh", "id_rsa"),
		meta.ClusterPath(clusterName, "ssh", "id_rsa.pub"))
	if err != nil {
		return perrs.AddStack(err)
	}

	err = ctx.SetClusterSSH(topo, metadata.User, gOpt.SSHTimeout)
	if err != nil {
		return perrs.AddStack(err)
	}

	filterRoles := set.NewStringSet(opt.Roles...)
	filterNodes := set.NewStringSet(opt.Nodes...)
	pdList := topo.GetPDList()
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

			status := ins.Status(pdList...)
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
				cliutil.OsArch(ins.OS(), ins.Arch()),
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
