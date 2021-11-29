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
	"fmt"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/tui"
)

// CleanCluster cleans the cluster without destroying it
func (m *Manager) CleanCluster(name string, gOpt operator.Options, cleanOpt operator.Options, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}
	// calculate file paths to be deleted before the prompt
	delFileMap := getCleanupFiles(topo, cleanOpt.CleanupData, cleanOpt.CleanupLog, false, cleanOpt.RetainDataRoles, cleanOpt.RetainDataNodes)

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

		// build file list string
		delFileList := ""
		for host, fileList := range delFileMap {
			delFileList += fmt.Sprintf("\n%s:", color.CyanString(host))
			for _, dfp := range fileList.Slice() {
				delFileList += fmt.Sprintf("\n %s", dfp)
			}
		}

		if err := tui.PromptForConfirmOrAbortError(
			"This operation will stop %s %s cluster %s and clean its' %s.\nNodes will be ignored: %s\nRoles will be ignored: %s\nFiles to be deleted are: %s\nDo you want to continue? [y/N]:",
			m.sysName,
			color.HiYellowString(base.Version),
			color.HiYellowString(name),
			target,
			cleanOpt.RetainDataNodes,
			cleanOpt.RetainDataRoles,
			delFileList); err != nil {
			return err
		}
		log.Infof("Cleanup cluster...")
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}
	t := b.
		Func("StopCluster", func(ctx context.Context) error {
			return operator.Stop(ctx, topo, operator.Options{}, tlsCfg)
		}).
		Func("CleanupCluster", func(ctx context.Context) error {
			return operator.CleanupComponent(ctx, delFileMap)
		}).
		Build()

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Cleanup cluster `%s` successfully", name)
	return nil
}
