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
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
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

	var (
		force bool     = gOpt.Force
		nodes []string = gOpt.Nodes
	)
	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			"This operation will delete the %s nodes in `%s` and all their data.\nDo you want to continue? [y/N]:",
			strings.Join(nodes, ","),
			color.HiYellowString(name)); err != nil {
			return err
		}

		if force {
			if err := cliutil.PromptForConfirmOrAbortError(
				"Forcing scale in is unsafe and may result in data lost for stateful components.\nDo you want to continue? [y/N]:",
			); err != nil {
				return err
			}
		}

		log.Infof("Scale-in nodes...")
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
	regenConfigTasks, hasImported := buildRegenConfigTasks(m, name, topo, base, nodes, true)

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

	b := m.sshTaskBuilder(name, topo, base.User, gOpt)

	scale(b, metadata, tlsCfg)

	t := b.
		ParallelStep("+ Refresh instance configs", force, regenConfigTasks...).
		Parallel(force, buildReloadPromTasks(metadata.GetTopology(), nodes...)...).
		Build()

	if err := t.Execute(ctxt.New(context.Background())); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Scaled cluster `%s` in successfully", name)

	return nil
}
