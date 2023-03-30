// Copyright 2023 PingCAP, Inc.
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

	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
)

// RotateSSH rotate public keys of target nodes
func (m *Manager) RotateSSH(name string, gOpt operator.Options, skipConfirm bool) error {
	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(
			"This operation will rotate ssh keys for user '%s' .\nDo you want to continue? [y/N]:",
			base.User); err != nil {
			return err
		}
	}

	var rotateSSHTasks []*task.StepDisplay // tasks which are used to initialize environment
	uniqueHosts, _ := getMonitorHosts(topo)
	for host, hostInfo := range uniqueHosts {
		t, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
		if err != nil {
			return err
		}
		t = t.RotateSSH(host, base.User, m.specManager.Path(name, "ssh", "new.pub"))

		rotateSSHTasks = append(rotateSSHTasks, t.BuildAsStep(fmt.Sprintf("  - Rotate ssh key on %s:%d", host, hostInfo.ssh)))
	}

	builder := task.NewBuilder(m.logger).
		Step("+ Generate new SSH keys",
			task.NewBuilder(m.logger).
				SSHKeyGen(m.specManager.Path(name, "ssh", "new")).
				Build(),
			m.logger).
		ParallelStep("+ rotate ssh keys of target host environments", false, rotateSSHTasks...).
		Step("+ overwrite old SSH keys",
			task.NewBuilder(m.logger).
				Func("rename", func(ctx context.Context) error {
					err := os.Rename(m.specManager.Path(name, "ssh", "new.pub"), m.specManager.Path(name, "ssh", "id_rsa.pub"))
					if err != nil {
						return err
					}
					err = os.Rename(m.specManager.Path(name, "ssh", "new"), m.specManager.Path(name, "ssh", "id_rsa"))
					if err != nil {
						return err
					}
					return nil
				}).
				Build(),
			m.logger)

	ctx := ctxt.New(
		context.Background(),
		gOpt.Concurrency,
		m.logger,
	)
	if err := builder.Build().Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return err
	}
	m.logger.Infof("ssh keys are successfully updated")
	return nil
}
