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
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
)

// EnableCluster enable/disable the service in a cluster
func (m *Manager) EnableCluster(name string, gOpt operator.Options, isEnable bool) error {
	if isEnable {
		log.Infof("Enabling cluster %s...", name)
	} else {
		log.Infof("Disabling cluster %s...", name)
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

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if isEnable {
		log.Infof("Enabled cluster `%s` successfully", name)
	} else {
		log.Infof("Disabled cluster `%s` successfully", name)
	}

	return nil
}

// StartCluster start the cluster with specified name.
func (m *Manager) StartCluster(name string, gOpt operator.Options, fn ...func(b *task.Builder, metadata spec.Metadata)) error {
	log.Infof("Starting cluster %s...", name)

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
		return operator.Start(ctx, topo, gOpt, tlsCfg)
	})

	for _, f := range fn {
		f(b, metadata)
	}

	t := b.Build()

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Started cluster `%s` successfully", name)
	return nil
}

// StopCluster stop the cluster.
func (m *Manager) StopCluster(name string, gOpt operator.Options, skipConfirm bool) error {
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
			return operator.Stop(ctx, topo, gOpt, tlsCfg)
		}).
		Build()

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Stopped cluster `%s` successfully", name)
	return nil
}

// RestartCluster restart the cluster.
func (m *Manager) RestartCluster(name string, gOpt operator.Options, skipConfirm bool) error {
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
			fmt.Sprintf("Will restart the cluster %s with nodes: %s roles: %s.\nDo you want to continue? [y/N]:",
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

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Restarted cluster `%s` successfully", name)
	return nil
}
