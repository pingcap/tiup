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
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
)

// ExecOptions for exec shell commanm.
type ExecOptions struct {
	Command string
	Sudo    bool
}

// Exec shell command on host in the tidb cluster.
func (m *Manager) Exec(name string, opt ExecOptions, gOpt operator.Options) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	filterRoles := set.NewStringSet(gOpt.Roles...)
	filterNodes := set.NewStringSet(gOpt.Nodes...)

	var shellTasks []task.Task
	uniqueHosts := map[string]set.StringSet{} // host-sshPort -> {command}
	topo.IterInstance(func(inst spec.Instance) {
		key := fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetSSHPort())
		if _, found := uniqueHosts[key]; !found {
			if len(gOpt.Roles) > 0 && !filterRoles.Exist(inst.Role()) {
				return
			}

			if len(gOpt.Nodes) > 0 && !filterNodes.Exist(inst.GetHost()) {
				return
			}

			cmds, err := renderInstanceSpec(opt.Command, inst)
			if err != nil {
				m.logger.Debugf("error rendering command with spec: %s", err)
				return // skip
			}
			cmdSet := set.NewStringSet(cmds...)
			if _, ok := uniqueHosts[key]; ok {
				uniqueHosts[key].Join(cmdSet)
				return
			}
			uniqueHosts[key] = cmdSet
		}
	})

	for hostKey, i := range uniqueHosts {
		host := strings.Split(hostKey, ":")[0]
		for _, cmd := range i.Slice() {
			shellTasks = append(shellTasks,
				task.NewBuilder(m.logger).
					Shell(host, cmd, hostKey+cmd, opt.Sudo).
					Build())
		}
	}

	b, err := m.sshTaskBuilder(name, topo, base.User, gOpt)
	if err != nil {
		return err
	}

	t := b.
		Parallel(false, shellTasks...).
		Build()

	execCtx := ctxt.New(
		context.Background(),
		gOpt.Concurrency,
		m.logger,
	)
	if err := t.Execute(execCtx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	// print outputs
	for hostKey, i := range uniqueHosts {
		host := strings.Split(hostKey, "-")[0]
		for _, cmd := range i.Slice() {
			stdout, stderr, ok := ctxt.GetInner(execCtx).GetOutputs(hostKey + cmd)
			if !ok {
				continue
			}
			m.logger.Infof("Outputs of %s on %s:",
				color.CyanString(cmd),
				color.CyanString(host))
			if len(stdout) > 0 {
				m.logger.Infof("%s:\n%s", color.GreenString("stdout"), stdout)
			}
			if len(stderr) > 0 {
				m.logger.Infof("%s:\n%s", color.RedString("stderr"), stderr)
			}
		}
	}

	return nil
}
