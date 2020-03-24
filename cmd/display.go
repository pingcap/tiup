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

func newDisplayCmd() *cobra.Command {
	var (
		clusterName string
		showStatus  bool
	)

	cmd := &cobra.Command{
		Use:    "display <cluster> [OPTIONS]",
		Short:  "Display information of a TiDB cluster",
		Hidden: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				cmd.Help()
				return fmt.Errorf("cluster name not specified")
			}
			clusterName = args[0]
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := displayClusterMeta(clusterName); err != nil {
				return err
			}
			return displayClusterTopology(clusterName, showStatus)
		},
	}

	cmd.Flags().BoolVarP(&showStatus, "status", "s", false, "test and show current node status")

	return cmd
}
func displayClusterMeta(name string) error {
	clsMeta, err := meta.ClusterMetadata(name)
	if err != nil {
		return err
	}

	cyan := color.New(color.FgCyan, color.Bold)

	fmt.Printf("TiDB Cluster: %s\n", cyan.Sprint(name))
	fmt.Printf("TiDB Version: %s\n", cyan.Sprint(clsMeta.Version))

	return nil
}

func displayClusterTopology(name string, showStatus bool) error {
	clsTopo, err := meta.ClusterTopology(name)
	if err != nil {
		return err
	}

	var clusterTable [][]string
	if showStatus {
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
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		subTable, err := buildTable(v.Field(i), showStatus)
		if err != nil {
			continue
		}
		clusterTable = append(clusterTable, subTable...)
	}

	utils.PrintTable(clusterTable, true)

	return nil
}

func buildTable(field reflect.Value, showStatus bool) ([][]string, error) {
	var resTable [][]string

	switch field.Kind() {
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			subTable, err := buildTable(field.Index(i), showStatus)
			if err != nil {
				return nil, err
			}
			resTable = append(resTable, subTable...)
		}
	case reflect.Ptr:
		subTable, err := buildTable(field.Elem(), showStatus)
		if err != nil {
			return nil, err
		}
		resTable = append(resTable, subTable...)
	case reflect.Struct:
		ins := field.Interface().(meta.InstanceSpec)

		dataDir := "-"
		insDirs := ins.GetDir()
		deployDir := insDirs[0]
		if len(insDirs) > 1 {
			dataDir = insDirs[1]
		}

		if showStatus {
			resTable = append(resTable, []string{
				color.CyanString(ins.GetID()),
				ins.Role(),
				ins.GetHost(),
				utils.JoinInt(ins.GetPort(), "/"),
				formatInstanceStatus(ins.GetStatus()),
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
	case "up":
		return color.GreenString(status)
	case "down", "offline", "tombstone":
		return color.RedString(status)
	default:
		return status
	}
}
