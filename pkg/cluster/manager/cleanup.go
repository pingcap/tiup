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
	"path"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
)

// CleanCluster cleans the cluster without destroying it
func (m *Manager) CleanCluster(name string, gOpt operator.Options, cleanOpt operator.Options, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// check locked
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
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
	delFileMap := getCleanupFiles(topo,
		cleanOpt.CleanupData, cleanOpt.CleanupLog, false, cleanOpt.CleanupAuditLog, cleanOpt.RetainDataRoles, cleanOpt.RetainDataNodes)

	if !skipConfirm {
		if err := cleanupConfirm(m.logger, name, m.sysName, base.Version, cleanOpt, delFileMap); err != nil {
			return err
		}
	}

	m.logger.Infof("Cleanup cluster...")

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}
	t := b.
		Func("StopCluster", func(ctx context.Context) error {
			return operator.Stop(
				ctx,
				topo,
				operator.Options{},
				false, /* eviceLeader */
				tlsCfg,
			)
		}).
		Func("CleanupCluster", func(ctx context.Context) error {
			return operator.CleanupComponent(ctx, delFileMap)
		}).
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

	m.logger.Infof("Cleanup%s in cluster `%s` successfully", cleanTarget(cleanOpt), name)
	return nil
}

// checkConfirm
func cleanupConfirm(logger *logprinter.Logger, clusterName, sysName, version string, cleanOpt operator.Options, delFileMap map[string]set.StringSet) error {
	logger.Warnf("The clean operation will %s %s %s cluster `%s`",
		color.HiYellowString("stop"), sysName, version, color.HiYellowString(clusterName))
	if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]:"); err != nil {
		return err
	}

	// build file list string
	delFileList := ""
	for host, fileList := range delFileMap {
		// target host has no files to delete
		if len(fileList) == 0 {
			continue
		}

		delFileList += fmt.Sprintf("\n%s:", color.CyanString(host))
		for _, dfp := range fileList.Slice() {
			delFileList += fmt.Sprintf("\n %s", dfp)
		}
	}

	logger.Warnf("Clean the clutser %s's%s.\nNodes will be ignored: %s\nRoles will be ignored: %s\nFiles to be deleted are: %s",
		color.HiYellowString(clusterName), cleanTarget(cleanOpt), cleanOpt.RetainDataNodes,
		cleanOpt.RetainDataRoles,
		delFileList)
	return tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]:")
}

func cleanTarget(cleanOpt operator.Options) string {
	target := ""

	if cleanOpt.CleanupData {
		target += " data"
	}

	if cleanOpt.CleanupLog {
		target += (" log")
	}

	if cleanOpt.CleanupAuditLog {
		target += (" audit-log")
	}

	return color.HiYellowString(target)
}

// cleanupFiles record the file that needs to be cleaned up
type cleanupFiles struct {
	cleanupData     bool     // whether to clean up the data
	cleanupLog      bool     // whether to clean up the log
	cleanupTLS      bool     // whether to clean up the tls files
	cleanupAuditLog bool     // whether to clean up the tidb server audit log
	retainDataRoles []string // roles that don't clean up
	retainDataNodes []string // roles that don't clean up
	ansibleImport   bool     // cluster is ansible deploy
	delFileMap      map[string]set.StringSet
}

// getCleanupFiles  get the files that need to be deleted
func getCleanupFiles(topo spec.Topology,
	cleanupData, cleanupLog, cleanupTLS, cleanupAuditLog bool, retainDataRoles, retainDataNodes []string) map[string]set.StringSet {
	c := &cleanupFiles{
		cleanupData:     cleanupData,
		cleanupLog:      cleanupLog,
		cleanupTLS:      cleanupTLS,
		cleanupAuditLog: cleanupAuditLog,
		retainDataRoles: retainDataRoles,
		retainDataNodes: retainDataNodes,
		delFileMap:      make(map[string]set.StringSet),
	}

	// calculate file paths to be deleted before the prompt
	c.instanceCleanupFiles(topo)
	c.monitorCleanupFiles(topo)

	return c.delFileMap
}

// instanceCleanupFiles get the files that need to be deleted in the component
func (c *cleanupFiles) instanceCleanupFiles(topo spec.Topology) {
	for _, com := range topo.ComponentsByStopOrder() {
		instances := com.Instances()
		retainDataRoles := set.NewStringSet(c.retainDataRoles...)
		retainDataNodes := set.NewStringSet(c.retainDataNodes...)

		for _, ins := range instances {
			// not cleaning files of monitor agents if the instance does not have one
			// may not work
			switch ins.ComponentName() {
			case spec.ComponentNodeExporter,
				spec.ComponentBlackboxExporter:
				if ins.IgnoreMonitorAgent() {
					continue
				}
			}

			// Some data of instances will be retained
			dataRetained := retainDataRoles.Exist(ins.ComponentName()) ||
				retainDataNodes.Exist(ins.ID()) || retainDataNodes.Exist(ins.GetHost())

			if dataRetained {
				continue
			}

			// prevent duplicate directories
			dataPaths := set.NewStringSet()
			logPaths := set.NewStringSet()
			tlsPath := set.NewStringSet()

			if c.cleanupData && len(ins.DataDir()) > 0 {
				for _, dataDir := range strings.Split(ins.DataDir(), ",") {
					dataPaths.Insert(path.Join(dataDir, "*"))
				}
			}

			if c.cleanupLog && len(ins.LogDir()) > 0 {
				for _, logDir := range strings.Split(ins.LogDir(), ",") {
					// need to judge the audit log of tidb server
					if ins.ComponentName() == spec.ComponentTiDB {
						logPaths.Insert(path.Join(logDir, "tidb?[!audit]*.log"))
						logPaths.Insert(path.Join(logDir, "tidb.log")) // maybe no need deleted
					} else {
						logPaths.Insert(path.Join(logDir, "*.log"))
					}
				}
			}

			if c.cleanupAuditLog && ins.ComponentName() == spec.ComponentTiDB {
				for _, logDir := range strings.Split(ins.LogDir(), ",") {
					logPaths.Insert(path.Join(logDir, "tidb-audit*.log"))
				}
			}

			// clean tls data
			if c.cleanupTLS && !topo.BaseTopo().GlobalOptions.TLSEnabled {
				deployDir := spec.Abs(topo.BaseTopo().GlobalOptions.User, ins.DeployDir())
				tlsDir := filepath.Join(deployDir, spec.TLSCertKeyDir)
				tlsPath.Insert(tlsDir)

				// ansible deploy
				if ins.IsImported() {
					ansibleTLSDir := filepath.Join(deployDir, spec.TLSCertKeyDirWithAnsible)
					tlsPath.Insert(ansibleTLSDir)
					c.ansibleImport = true
				}
			}

			if c.delFileMap[ins.GetHost()] == nil {
				c.delFileMap[ins.GetHost()] = set.NewStringSet()
			}
			c.delFileMap[ins.GetHost()].Join(logPaths).Join(dataPaths).Join(tlsPath)
		}
	}
}

// monitorCleanupFiles get the files that need to be deleted in the mointor
func (c *cleanupFiles) monitorCleanupFiles(topo spec.Topology) {
	monitoredOptions := topo.BaseTopo().MonitoredOptions
	if monitoredOptions == nil {
		return
	}
	user := topo.BaseTopo().GlobalOptions.User

	// get the host with monitor installed
	uniqueHosts, noAgentHosts := getMonitorHosts(topo)
	retainDataNodes := set.NewStringSet(c.retainDataNodes...)

	// monitoring agents
	for host := range uniqueHosts {
		// determine if host don't need to delete
		dataRetained := noAgentHosts.Exist(host) || retainDataNodes.Exist(host)
		if dataRetained {
			continue
		}

		deployDir := spec.Abs(user, monitoredOptions.DeployDir)

		// prevent duplicate directories
		dataPaths := set.NewStringSet()
		logPaths := set.NewStringSet()
		tlsPath := set.NewStringSet()

		// data dir would be empty for components which don't need it
		dataDir := monitoredOptions.DataDir
		if c.cleanupData && len(dataDir) > 0 {
			// the default data_dir is relative to deploy_dir
			if !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join(deployDir, dataDir)
			}
			dataPaths.Insert(path.Join(dataDir, "*"))
		}

		// log dir will always be with values, but might not used by the component
		logDir := spec.Abs(user, monitoredOptions.LogDir)
		if c.cleanupLog && len(logDir) > 0 {
			logPaths.Insert(path.Join(logDir, "*.log"))
		}

		// clean tls data
		if c.cleanupTLS && !topo.BaseTopo().GlobalOptions.TLSEnabled {
			tlsDir := filepath.Join(deployDir, spec.TLSCertKeyDir)
			tlsPath.Insert(tlsDir)
			// ansible deploy
			if c.ansibleImport {
				ansibleTLSDir := filepath.Join(deployDir, spec.TLSCertKeyDirWithAnsible)
				tlsPath.Insert(ansibleTLSDir)
			}
		}

		if c.delFileMap[host] == nil {
			c.delFileMap[host] = set.NewStringSet()
		}
		c.delFileMap[host].Join(logPaths).Join(dataPaths).Join(tlsPath)
	}
}
