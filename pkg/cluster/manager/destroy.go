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

package manager

import (
	"context"
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
)

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyCluster(name string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(
			"This operation will destroy %s %s cluster %s and its data.\nDo you want to continue? [y/N]:",
			m.sysName,
			color.HiYellowString(base.Version),
			color.HiYellowString(name)); err != nil {
			return err
		}
		log.Infof("Destroying cluster...")
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}
	t := b.
		Func("StopCluster", func(ctx context.Context) error {
			return operator.Stop(ctx, topo, operator.Options{Force: destroyOpt.Force}, tlsCfg)
		}).
		Func("DestroyCluster", func(ctx context.Context) error {
			return operator.Destroy(ctx, topo, destroyOpt)
		}).
		Build()

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if err := m.specManager.Remove(name); err != nil {
		return perrs.Trace(err)
	}

	log.Infof("Destroyed cluster `%s` successfully", name)
	return nil
}

// DestroyTombstone destroy and remove instances that is in tombstone state
func (m *Manager) DestroyTombstone(
	name string,
	gOpt operator.Options,
	skipConfirm bool,
) error {
	metadata, err := m.meta(name)
	// allow specific validation errors so that user can recover a broken
	// cluster if it is somehow in a bad state.
	if err != nil &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	clusterMeta := metadata.(*spec.ClusterMeta)
	cluster := clusterMeta.Topology

	if !operator.NeedCheckTombstone(cluster) {
		return nil
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}

	ctx := ctxt.New(context.Background(), gOpt.Concurrency)
	nodes, err := operator.DestroyTombstone(ctx, cluster, true /* returnNodesOnly */, gOpt, tlsCfg)
	if err != nil {
		return err
	}
	regenConfigTasks, _ := buildRegenConfigTasks(m, name, topo, base, nodes, true)

	t := b.
		Func("FindTomestoneNodes", func(ctx context.Context) (err error) {
			if !skipConfirm {
				err = tui.PromptForConfirmOrAbortError(
					color.HiYellowString(fmt.Sprintf("Will destroy these nodes: %v\nDo you confirm this action? [y/N]:", nodes)),
				)
				if err != nil {
					return err
				}
			}
			log.Infof("Start destroy Tombstone nodes: %v ...", nodes)
			return err
		}).
		ClusterOperate(cluster, operator.DestroyTombstoneOperation, gOpt, tlsCfg).
		UpdateMeta(name, clusterMeta, nodes).
		UpdateTopology(name, m.specManager.Path(name), clusterMeta, nodes).
		ParallelStep("+ Refresh instance configs", true, regenConfigTasks...).
		Parallel(true, buildReloadPromTasks(metadata.GetTopology())...).
		Build()

	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Destroy success")

	return nil
}
