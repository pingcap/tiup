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

package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/spf13/cobra"
)

type displayOption struct {
	clusterName string
	filterRole  []string
	filterNode  []string
}

func newDisplayCmd() *cobra.Command {
	opt := displayOption{}

	cmd := &cobra.Command{
		Use:   "display <cluster> [OPTIONS]",
		Short: "Display information of a TiDB cluster",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			opt.clusterName = args[0]
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := displayClusterMeta(&opt); err != nil {
				return err
			}
			return displayClusterTopology(&opt)
		},
	}

	cmd.Flags().StringSliceVarP(&opt.filterRole, "role", "R", nil, "Only display specified roles")
	cmd.Flags().StringSliceVarP(&opt.filterNode, "node", "N", nil, "Only display specified nodes")

	return cmd
}

func displayClusterMeta(opt *displayOption) error {
	clsMeta, err := meta.ClusterMetadata(opt.clusterName)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold)

	log.Output(fmt.Sprintf("TiDB Cluster: %s", cyan.Sprint(opt.clusterName)))
	log.Output(fmt.Sprintf("TiDB Version: %s", cyan.Sprint(clsMeta.Version)))

	return nil
}

func displayClusterTopology(opt *displayOption) error {
	topo, err := meta.ClusterTopology(opt.clusterName)
	if err != nil {
		return err
	}

	clusterTable := [][]string{
		// Header
		{"ID", "Role", "Host", "Ports", "Status", "Data Dir", "Deploy Dir"},
	}

	filterRoles := set.NewStringSet(opt.filterRole...)
	filterNodes := set.NewStringSet(opt.filterNode...)
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

			clusterTable = append(clusterTable, []string{
				color.CyanString(ins.ID()),
				ins.Role(),
				ins.GetHost(),
				utils.JoinInt(ins.UsedPorts(), "/"),
				formatInstanceStatus(ins.Status(pdList...)),
				dataDir,
				deployDir,
			})

		}
	}

	utils.PrintTable(clusterTable, true)

	return nil
}

func formatInstanceStatus(status string) string {
	switch strings.ToLower(status) {
	case "up", "healthy":
		return color.GreenString(status)
	case "healthy|l": // PD leader
		return color.HiGreenString(status)
	case "offline", "tombstone":
		return color.YellowString(status)
	case "down", "unhealthy", "err":
		return color.RedString(status)
	default:
		return status
	}
}
