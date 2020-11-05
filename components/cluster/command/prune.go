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

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/spf13/cobra"
)

func newPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune <cluster-name>",
		Short: "Destroy and remove instances that is in tombstone state",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]

			metadata, err := spec.ClusterMetadata(clusterName)
			if err != nil {
				return err
			}

			return destroyTombstoneIfNeed(clusterName, metadata, gOpt, skipConfirm)
		},
	}

	return cmd
}

func destroyTombstoneIfNeed(clusterName string, metadata *spec.ClusterMeta, opt operator.Options, skipConfirm bool) error {
	topo := metadata.Topology

	if !operator.NeedCheckTombstone(topo) {
		return nil
	}

	tlsCfg, err := topo.TLSConfig(tidbSpec.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}

	ctx := task.NewContext()
	err = ctx.SetSSHKeySet(spec.ClusterPath(clusterName, "ssh", "id_rsa"),
		spec.ClusterPath(clusterName, "ssh", "id_rsa.pub"))
	if err != nil {
		return perrs.AddStack(err)
	}

	err = ctx.SetClusterSSH(topo, metadata.User, gOpt.SSHTimeout, gOpt.SSHType, topo.BaseTopo().GlobalOptions.SSHType)
	if err != nil {
		return perrs.AddStack(err)
	}

	nodes, err := operator.DestroyTombstone(ctx, topo, true /* returnNodesOnly */, opt, tlsCfg)
	if err != nil {
		return perrs.AddStack(err)
	}

	if len(nodes) == 0 {
		return nil
	}

	if !skipConfirm {
		err = cliutil.PromptForConfirmOrAbortError(
			color.HiYellowString(fmt.Sprintf("Will destroy these nodes: %v\nDo you confirm this action? [y/N]:", nodes)),
		)
		if err != nil {
			return err
		}
	}

	log.Infof("Start destroy Tombstone nodes: %v ...", nodes)

	_, err = operator.DestroyTombstone(ctx, topo, false /* returnNodesOnly */, opt, tlsCfg)
	if err != nil {
		return perrs.AddStack(err)
	}

	log.Infof("Destroy success")

	return spec.SaveClusterMeta(clusterName, metadata)
}
