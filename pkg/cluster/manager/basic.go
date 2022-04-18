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
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

// EnableCluster enable/disable the service in a cluster
func (m *Manager) EnableCluster(name string, gOpt operator.Options, isEnable bool) error {
	if isEnable {
		m.logger.Infof("Enabling cluster %s...", name)
	} else {
		m.logger.Infof("Disabling cluster %s...", name)
	}

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}

	if isEnable {
		b = b.Func("EnableCluster", func(ctx context.Context) error {
			return operator.Enable(ctx, topo, gOpt, isEnable)
		})
	} else {
		b = b.Func("DisableCluster", func(ctx context.Context) error {
			return operator.Enable(ctx, topo, gOpt, isEnable)
		})
	}

	t := b.Build()

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

	if isEnable {
		m.logger.Infof("Enabled cluster `%s` successfully", name)
	} else {
		m.logger.Infof("Disabled cluster `%s` successfully", name)
	}

	return nil
}

// StartCluster start the cluster with specified name.
func (m *Manager) StartCluster(name string, gOpt operator.Options, restoreLeader bool, fn ...func(b *task.Builder, metadata spec.Metadata)) error {
	m.logger.Infof("Starting cluster %s...", name)

	// check locked
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}

	b.Func("StartCluster", func(ctx context.Context) error {
		return operator.Start(ctx, topo, gOpt, restoreLeader, tlsCfg)
	})

	for _, f := range fn {
		f(b, metadata)
	}

	t := b.Build()

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

	m.logger.Infof("Started cluster `%s` successfully", name)
	return nil
}

// StopCluster stop the cluster.
func (m *Manager) StopCluster(
	name string,
	gOpt operator.Options,
	skipConfirm,
	evictLeader bool,
) error {
	// check locked
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
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
			fmt.Sprintf("Will stop the cluster %s with nodes: %s, roles: %s.\nDo you want to continue? [y/N]:",
				color.HiYellowString(name),
				color.HiRedString(strings.Join(gOpt.Nodes, ",")),
				color.HiRedString(strings.Join(gOpt.Roles, ",")),
			),
		); err != nil {
			return err
		}
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}

	t := b.
		Func("StopCluster", func(ctx context.Context) error {
			return operator.Stop(ctx, topo, gOpt, evictLeader, tlsCfg)
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

	m.logger.Infof("Stopped cluster `%s` successfully", name)
	return nil
}

// RestartCluster restart the cluster.
func (m *Manager) RestartCluster(name string, gOpt operator.Options, skipConfirm bool) error {
	// check locked
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
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
			fmt.Sprintf("Will restart the cluster %s with nodes: %s roles: %s.\nCluster will be unavailable\nDo you want to continue? [y/N]:",
				color.HiYellowString(name),
				color.HiYellowString(strings.Join(gOpt.Nodes, ",")),
				color.HiYellowString(strings.Join(gOpt.Roles, ",")),
			),
		); err != nil {
			return err
		}
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}
	t := b.
		Func("RestartCluster", func(ctx context.Context) error {
			return operator.Restart(ctx, topo, gOpt, tlsCfg)
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

	m.logger.Infof("Restarted cluster `%s` successfully", name)
	return nil
}

// getMonitorHosts  get the instance to ignore list if it marks itself as ignore_exporter
func getMonitorHosts(topo spec.Topology) (map[string]hostInfo, set.StringSet) {
	// monitor
	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	noAgentHosts := set.NewStringSet()
	topo.IterInstance(func(inst spec.Instance) {
		// add the instance to ignore list if it marks itself as ignore_exporter
		if inst.IgnoreMonitorAgent() {
			noAgentHosts.Insert(inst.GetHost())
		}

		if _, found := uniqueHosts[inst.GetHost()]; !found {
			uniqueHosts[inst.GetHost()] = hostInfo{
				ssh:  inst.GetSSHPort(),
				os:   inst.OS(),
				arch: inst.Arch(),
			}
		}
	})

	return uniqueHosts, noAgentHosts
}

// checkTiFlashWithTLS check tiflash vserson
func checkTiFlashWithTLS(topo spec.Topology, version string) error {
	if clusterSpec, ok := topo.(*spec.Specification); ok {
		if clusterSpec.GlobalOptions.TLSEnabled {
			if (semver.Compare(version, "v4.0.5") < 0 &&
				len(clusterSpec.TiFlashServers) > 0) &&
				version != utils.NightlyVersionAlias {
				return fmt.Errorf("TiFlash %s is not supported in TLS enabled cluster", version)
			}
		}
	}
	return nil
}

// BackupClusterMeta backup cluster meta to given filepath
func (m *Manager) BackupClusterMeta(clusterName, filePath string) error {
	exist, err := m.specManager.Exist(clusterName)
	if err != nil {
		return err
	}
	if !exist {
		return fmt.Errorf("cluster %s does not exist", clusterName)
	}
	// check locked
	if err := m.specManager.ScaleOutLockedErr(clusterName); err != nil {
		return err
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	return utils.Tar(f, m.specManager.Path(clusterName))
}

// RestoreClusterMeta restore cluster meta by given filepath
func (m *Manager) RestoreClusterMeta(clusterName, filePath string, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(clusterName); err != nil {
		return err
	}

	fi, err := os.Stat(m.specManager.Path(clusterName))
	if err != nil {
		if !os.IsNotExist(err) {
			return perrs.AddStack(err)
		}
		m.logger.Infof(fmt.Sprintf("meta of cluster %s didn't exist before restore", clusterName))
		skipConfirm = true
	} else {
		m.logger.Warnf(color.HiRedString(tui.ASCIIArtWarning))

		exist, err := m.specManager.Exist(clusterName)
		if err != nil {
			return err
		}
		if exist {
			m.logger.Infof(fmt.Sprintf("the exist meta.yaml of cluster %s was last modified at %s", clusterName, color.HiYellowString(fi.ModTime().Format(time.RFC3339))))
		} else {
			m.logger.Infof(fmt.Sprintf("the meta.yaml of cluster %s does not exist", clusterName))
		}
	}
	fi, err = os.Stat(filePath)
	if err != nil {
		return err
	}
	m.logger.Warnf(fmt.Sprintf("the given tarball was last modified at %s", color.HiYellowString(fi.ModTime().Format(time.RFC3339))))
	if !skipConfirm {
		if err := tui.PromptForAnswerOrAbortError(
			"Yes, I know my cluster meta will be be overridden.",
			fmt.Sprintf("This operation will override topology file and other meta file of %s cluster %s .",
				m.sysName,
				color.HiYellowString(clusterName),
			)+"\nAre you sure to continue?",
		); err != nil {
			return err
		}
		m.logger.Infof("Destroying cluster...")
	}
	err = os.RemoveAll(m.specManager.Path(clusterName))
	if err != nil {
		return err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	err = utils.Untar(f, m.specManager.Path(clusterName))
	if err == nil {
		m.logger.Infof(fmt.Sprintf("restore meta of cluster %s successfully.", clusterName))
	}
	return err
}
