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
	"fmt"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
)

// CleanCluster cleans the cluster without destroying it
func (m *Manager) CleanCluster(name string, gOpt operator.Options, cleanOpt operator.Options, skipConfirm bool) error {
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
	delFileMap := make(map[string]set.StringSet)
	for _, com := range topo.ComponentsByStopOrder() {
		instances := com.Instances()
		retainDataRoles := set.NewStringSet(cleanOpt.RetainDataRoles...)
		retainDataNodes := set.NewStringSet(cleanOpt.RetainDataNodes...)

		for _, ins := range instances {
			// Some data of instances will be retained
			dataRetained := retainDataRoles.Exist(ins.ComponentName()) ||
				retainDataNodes.Exist(ins.ID()) || retainDataNodes.Exist(ins.GetHost())

			if dataRetained {
				continue
			}

			dataPaths := set.NewStringSet()
			logPaths := set.NewStringSet()

			if cleanOpt.CleanupData && len(ins.DataDir()) > 0 {
				for _, dataDir := range strings.Split(ins.DataDir(), ",") {
					dataPaths.Insert(path.Join(dataDir, "*"))
				}
			}

			if cleanOpt.CleanupLog && len(ins.LogDir()) > 0 {
				for _, logDir := range strings.Split(ins.LogDir(), ",") {
					logPaths.Insert(path.Join(logDir, "*.log"))
				}
			}

			if delFileMap[ins.GetHost()] == nil {
				delFileMap[ins.GetHost()] = set.NewStringSet()
			}
			delFileMap[ins.GetHost()].Join(logPaths).Join(dataPaths)
		}
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

		// build file list string
		delFileList := ""
		for host, fileList := range delFileMap {
			delFileList += fmt.Sprintf("\n%s:", color.CyanString(host))
			for _, dfp := range fileList.Slice() {
				delFileList += fmt.Sprintf("\n %s", dfp)
			}
		}

		if err := cliutil.PromptForConfirmOrAbortError(
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

	t := m.sshTaskBuilder(name, topo, base.User, gOpt).
		Func("StopCluster", func(ctx *task.Context) error {
			return operator.Stop(ctx, topo, operator.Options{}, tlsCfg)
		}).
		Func("CleanupCluster", func(ctx *task.Context) error {
			return operator.CleanupComponent(ctx, delFileMap)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Cleanup cluster `%s` successfully", name)
	return nil
}
