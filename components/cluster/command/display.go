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
	"time"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
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

			exist, err := tidbSpec.Exist(clusterName)
			if err != nil {
				return perrs.AddStack(err)
			}

			if !exist {
				return perrs.Errorf("Cluster %s not found", clusterName)
			}

			if showDashboardOnly {
				return displayDashboardInfo(clusterName)
			}

			err = manager.Display(clusterName, gOpt)
			if err != nil {
				return perrs.AddStack(err)
			}

			metadata, err := spec.ClusterMetadata(clusterName)
			if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
				!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
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
	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
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

func destroyTombstoneIfNeed(clusterName string, metadata *spec.ClusterMeta, opt operator.Options) error {
	topo := metadata.Topology

	if !operator.NeedCheckTomebsome(topo) {
		return nil
	}

	ctx := task.NewContext()
	err := ctx.SetSSHKeySet(spec.ClusterPath(clusterName, "ssh", "id_rsa"),
		spec.ClusterPath(clusterName, "ssh", "id_rsa.pub"))
	if err != nil {
		return perrs.AddStack(err)
	}

	err = ctx.SetClusterSSH(topo, metadata.User, gOpt.SSHTimeout, gOpt.NativeSSH)
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

	return spec.SaveClusterMeta(clusterName, metadata)
}
