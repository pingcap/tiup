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
	"crypto/tls"
	"errors"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
)

// ScaleIn the cluster.
func (m *Manager) ScaleIn(
	name string,
	skipConfirm bool,
	gOpt operator.Options,
	scale func(builer *task.Builder, metadata spec.Metadata, tlsCfg *tls.Config),
) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// check locked
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
		return err
	}

	var (
		force bool     = gOpt.Force
		nodes []string = gOpt.Nodes
	)
	if !skipConfirm {
		if force {
			m.logger.Warnf(color.HiRedString(tui.ASCIIArtWarning))
			if err := tui.PromptForAnswerOrAbortError(
				"Yes, I know my data might be lost.",
				color.HiRedString("Forcing scale in is unsafe and may result in data loss for stateful components.\n"+
					"DO NOT use `--force` if you have any component in ")+
					color.YellowString("Pending Offline")+color.HiRedString(" status.\n")+
					color.HiRedString("The process is irreversible and could NOT be cancelled.\n")+
					"Only use `--force` when some of the servers are already permanently offline.\n"+
					"Are you sure to continue?",
			); err != nil {
				return err
			}
		}

		if err := tui.PromptForConfirmOrAbortError(
			"This operation will delete the %s nodes in `%s` and all their data.\nDo you want to continue? [y/N]:",
			strings.Join(nodes, ","),
			color.HiYellowString(name)); err != nil {
			return err
		}

		m.logger.Infof("Scale-in nodes...")
	}

	metadata, err := m.meta(name)
	if err != nil &&
		!errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTiSparkMaster) &&
		!errors.Is(perrs.Cause(err), spec.ErrMultipleTisparkWorker) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		// ignore conflict check error, node may be deployed by former version
		// that lack of some certain conflict checks
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// Regenerate configuration
	gOpt.IgnoreConfigCheck = true
	regenConfigTasks, hasImported := buildInitConfigTasks(m, name, topo, base, gOpt, nodes)

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(name); err != nil {
			return err
		}
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}

	scale(b, metadata, tlsCfg)

	t := b.
		ParallelStep("+ Refresh instance configs", force, regenConfigTasks...).
		ParallelStep("+ Reloda prometheus and grafana", gOpt.Force,
			buildReloadPromAndGrafanaTasks(metadata.GetTopology(), m.logger, gOpt, nodes...)...).
		Build()

	ctx := ctxt.New(
		context.Background(),
		gOpt.Concurrency,
		m.logger,
	)
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	m.logger.Infof("Scaled cluster `%s` in successfully", name)

	return nil
}
