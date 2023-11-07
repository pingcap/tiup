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
	"strings"
	"time"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/manager"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

func newDisplayCmd() *cobra.Command {
	var (
		showDashboardOnly bool
		showVersionOnly   bool
		showTiKVLabels    bool
		statusTimeout     uint64
		dopt              manager.DisplayOption
	)
	cmd := &cobra.Command{
		Use:   "display <cluster-name>",
		Short: "Display information of a TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			gOpt.APITimeout = statusTimeout
			dopt.ClusterName = args[0]
			clusterReport.ID = scrubClusterName(dopt.ClusterName)
			teleCommand = append(teleCommand, scrubClusterName(dopt.ClusterName))

			exist, err := tidbSpec.Exist(dopt.ClusterName)
			if err != nil {
				return err
			}

			if !exist {
				return perrs.Errorf("Cluster %s not found", dopt.ClusterName)
			}

			metadata, err := spec.ClusterMetadata(dopt.ClusterName)
			if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
				!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
				return err
			}

			if showVersionOnly {
				fmt.Println(metadata.Version)
				return nil
			}

			if showDashboardOnly {
				tlsCfg, err := metadata.Topology.TLSConfig(tidbSpec.Path(dopt.ClusterName, spec.TLSCertKeyDir))
				if err != nil {
					return err
				}
				return cm.DisplayDashboardInfo(dopt.ClusterName, time.Second*time.Duration(gOpt.APITimeout), tlsCfg)
			}
			if showTiKVLabels {
				return cm.DisplayTiKVLabels(dopt, gOpt)
			}
			return cm.Display(dopt, gOpt)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return shellCompGetClusterName(cm, toComplete)
			default:
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only display specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only display specified nodes")
	cmd.Flags().BoolVar(&dopt.ShowUptime, "uptime", false, "Display with uptime")
	cmd.Flags().BoolVar(&showDashboardOnly, "dashboard", false, "Only display TiDB Dashboard information")
	cmd.Flags().BoolVar(&showVersionOnly, "version", false, "Only display TiDB cluster version")
	cmd.Flags().BoolVar(&showTiKVLabels, "labels", false, "Only display labels of specified TiKV role or nodes")
	cmd.Flags().BoolVar(&dopt.ShowProcess, "process", false, "display cpu and memory usage of nodes")
	cmd.Flags().BoolVar(&dopt.ShowManageHost, "manage-host", false, "display manage host of nodes")
	cmd.Flags().BoolVar(&dopt.ShowNuma, "numa", false, "display numa information of nodes")
	cmd.Flags().BoolVar(&dopt.ShowVersions, "versions", false, "display component version of instances")
	cmd.Flags().Uint64Var(&statusTimeout, "status-timeout", 10, "Timeout in seconds when getting node status")

	return cmd
}

func shellCompGetClusterName(cm *manager.Manager, toComplete string) ([]string, cobra.ShellCompDirective) {
	var result []string
	clusters, _ := cm.GetClusterList()
	for _, c := range clusters {
		if strings.HasPrefix(c.Name, toComplete) {
			result = append(result, c.Name)
		}
	}
	return result, cobra.ShellCompDirectiveNoFileComp
}
