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
	"time"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

func newDisplayCmd() *cobra.Command {
	var (
		clusterName       string
		showDashboardOnly bool
		showVersionOnly   bool
		showTiKVLabels    bool
		statusTimeout     uint64
	)
	cmd := &cobra.Command{
		Use:   "display <cluster-name>",
		Short: "Display information of a TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			gOpt.APITimeout = statusTimeout
			clusterName = args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			exist, err := tidbSpec.Exist(clusterName)
			if err != nil {
				return err
			}

			if !exist {
				return perrs.Errorf("Cluster %s not found", clusterName)
			}

			metadata, err := spec.ClusterMetadata(clusterName)
			if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
				!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
				return err
			}

			if showVersionOnly {
				fmt.Println(metadata.Version)
				return nil
			}

			if showDashboardOnly {
				tlsCfg, err := metadata.Topology.TLSConfig(tidbSpec.Path(clusterName, spec.TLSCertKeyDir))
				if err != nil {
					return err
				}
				return cm.DisplayDashboardInfo(clusterName, time.Second*time.Duration(gOpt.APITimeout), tlsCfg)
			}
			if showTiKVLabels {
				return cm.DisplayTiKVLabels(clusterName, gOpt)
			}
			return cm.Display(clusterName, gOpt)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only display specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only display specified nodes")
	cmd.Flags().BoolVar(&gOpt.ShowUptime, "uptime", false, "Display with uptime")
	cmd.Flags().BoolVar(&showDashboardOnly, "dashboard", false, "Only display TiDB Dashboard information")
	cmd.Flags().BoolVar(&showVersionOnly, "version", false, "Only display TiDB cluster version")
	cmd.Flags().BoolVar(&showTiKVLabels, "labels", false, "Only display labels of specified TiKV role or nodes")
	cmd.Flags().Uint64Var(&statusTimeout, "status-timeout", 10, "Timeout in seconds when getting node status")

	return cmd
}
