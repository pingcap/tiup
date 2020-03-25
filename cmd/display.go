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
	"reflect"
	"strings"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/spf13/cobra"
)

type displayOption struct {
	clusterName string
	showStatus  bool
	filterRole  []string
	filterNode  []string
}

func newDisplayCmd() *cobra.Command {
	opt := displayOption{}

	cmd := &cobra.Command{
		Use:   "display <cluster> [OPTIONS]",
		Short: "Display information of a TiDB cluster",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				cmd.Help()
				return fmt.Errorf("cluster name not specified")
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

	cmd.Flags().BoolVarP(&opt.showStatus, "status", "s", false, "test and show current node status")
	cmd.Flags().StringSliceVar(&opt.filterRole, "role", nil, "only display nodes of specific roles")
	cmd.Flags().StringSliceVar(&opt.filterNode, "node", nil, "only display nodes of specific IDs")

	return cmd
}
func displayClusterMeta(opt *displayOption) error {
	clsMeta, err := meta.ClusterMetadata(opt.clusterName)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold)

	fmt.Printf("TiDB Cluster: %s\n", cyan.Sprint(opt.clusterName))
	fmt.Printf("TiDB Version: %s\n", cyan.Sprint(clsMeta.Version))

	return nil
}

func displayClusterTopology(opt *displayOption) error {
	clsTopo, err := meta.ClusterTopology(opt.clusterName)
	if err != nil {
		return err
	}

	var clusterTable [][]string
	if opt.showStatus {
		clusterTable = append(clusterTable,
			[]string{"ID",
				"Role",
				"Host",
				"Ports",
				"Status",
				"Data Dir",
				"Deploy Dir"})
	} else {
		clusterTable = append(clusterTable,
			[]string{"ID",
				"Role",
				"Host",
				"Ports",
				"Data Dir",
				"Deploy Dir"})
	}

	v := reflect.ValueOf(*clsTopo)
	pdList := clsTopo.GetPDList()
	for i := 0; i < v.NumField(); i++ {
		subTable, err := buildTable(v.Field(i), opt, pdList)
		if err != nil {
			continue
		}
		clusterTable = append(clusterTable, subTable...)
	}

	utils.PrintTable(clusterTable, true)

	return nil
}

func buildTable(field reflect.Value, opt *displayOption, pdList []string) ([][]string, error) {
	var resTable [][]string

	switch field.Kind() {
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			subTable, err := buildTable(field.Index(i), opt, pdList)
			if err != nil {
				return nil, err
			}
			resTable = append(resTable, subTable...)
		}
	case reflect.Ptr:
		subTable, err := buildTable(field.Elem(), opt, pdList)
		if err != nil {
			return nil, err
		}
		resTable = append(resTable, subTable...)
	case reflect.Struct:
		ins := field.Interface().(meta.InstanceSpec)

		// filter by role
		if opt.filterRole != nil && !utils.InSlice(ins.Role(), opt.filterRole) {
			return nil, nil
		}
		// filter by node
		if opt.filterNode != nil && !utils.InSlice(ins.GetID(), opt.filterNode) {
			return nil, nil
		}

		dataDir := "-"
		insDirs := ins.GetDir()
		deployDir := insDirs[0]
		if len(insDirs) > 1 {
			dataDir = insDirs[1]
		}

		if opt.showStatus {
			resTable = append(resTable, []string{
				color.CyanString(ins.GetID()),
				ins.Role(),
				ins.GetHost(),
				utils.JoinInt(ins.GetPort(), "/"),
				formatInstanceStatus(ins.GetStatus(pdList...)),
				dataDir,
				deployDir,
			})
		} else {
			resTable = append(resTable, []string{
				color.CyanString(ins.GetID()),
				ins.Role(),
				ins.GetHost(),
				utils.JoinInt(ins.GetPort(), "/"),
				dataDir,
				deployDir,
			})
		}
	}

	return resTable, nil
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
