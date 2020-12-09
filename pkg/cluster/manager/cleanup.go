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
	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
)

// CleanCluster cleans the cluster without destroying it
func (m *Manager) CleanCluster(clusterName string, gOpt operator.Options, cleanOpt operator.Options, skipConfirm bool) error {
	metadata, err := m.meta(clusterName)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	if !skipConfirm {
		target := ""
		switch {
		case cleanOpt.CleanupData && cleanOpt.CleanupLog:
			target = "data and log"
		case cleanOpt.CleanupData:
			target = "data"
		case cleanOpt.CleanupLog:
			target = "log"
		}
		if err := cliutil.PromptForConfirmOrAbortError(
			"This operation will clean %s %s cluster %s's %s.\nNodes will be ignored: %s\nRoles will be ignored: %s\nDo you want to continue? [y/N]:",
			m.sysName,
			color.HiYellowString(base.Version),
			color.HiYellowString(clusterName),
			target,
			cleanOpt.RetainDataNodes,
			cleanOpt.RetainDataRoles); err != nil {
			return err
		}
		log.Infof("Cleanup cluster...")
	}

	t := m.sshTaskBuilder(clusterName, topo, base.User, gOpt).
		Func("StopCluster", func(ctx *task.Context) error {
			return operator.Stop(ctx, topo, operator.Options{}, tlsCfg)
		}).
		Func("CleanupCluster", func(ctx *task.Context) error {
			return operator.Cleanup(ctx, topo, cleanOpt)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Cleanup cluster `%s` successfully", clusterName)
	return nil
}
