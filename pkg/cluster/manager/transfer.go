// Copyright 2021 PingCAP, Inc.
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
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
)

// TransferOptions for exec shell commanm.
type TransferOptions struct {
	Local  string
	Remote string
	Pull   bool // default to push
}

// Transfer copies files from or to host in the tidb cluster.
func (m *Manager) Transfer(name string, opt TransferOptions, gOpt operator.Options) error {
	metadata, err := m.meta(name)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	filterRoles := set.NewStringSet(gOpt.Roles...)
	filterNodes := set.NewStringSet(gOpt.Nodes...)

	var shellTasks []task.Task

	uniqueHosts := map[string]set.StringSet{} // host-sshPort-port -> {remote-path}
	topo.IterInstance(func(inst spec.Instance) {
		key := fmt.Sprintf("%s-%d-%d", inst.GetHost(), inst.GetSSHPort(), inst.GetPort())
		if _, found := uniqueHosts[key]; !found {
			if len(gOpt.Roles) > 0 && !filterRoles.Exist(inst.Role()) {
				return
			}

			if len(gOpt.Nodes) > 0 && !filterNodes.Exist(inst.GetHost()) {
				return
			}

			// render remote path
			instPath := opt.Remote
			paths, err := renderInstanceSpec(instPath, inst)
			if err != nil {
				return // skip
			}
			pathSet := set.NewStringSet(paths...)
			if _, ok := uniqueHosts[key]; ok {
				uniqueHosts[key].Join(pathSet)
				return
			}
			uniqueHosts[key] = pathSet
		}
	})

	srcPath := opt.Local
	for hostKey, i := range uniqueHosts {
		host := strings.Split(hostKey, "-")[0]
		for _, p := range i.Slice() {
			t := task.NewBuilder()
			if opt.Pull {
				t.CopyFile(p, srcPath, host, opt.Pull)
			} else {
				t.CopyFile(srcPath, p, host, opt.Pull)
			}
			shellTasks = append(shellTasks, t.Build())
		}
	}

	t := m.sshTaskBuilder(name, topo, base.User, gOpt).
		Parallel(false, shellTasks...).
		Build()

	execCtx := task.NewContext()
	if err := t.Execute(execCtx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	return nil
}

func renderInstanceSpec(t string, inst spec.Instance) ([]string, error) {
	result := make([]string, 0)
	switch inst.ComponentName() {
	case spec.ComponentTiFlash:
		for _, d := range strings.Split(inst.DataDir(), ",") {
			tf := inst
			tfs, ok := tf.(*spec.TiFlashInstance).InstanceSpec.(spec.TiFlashSpec)
			if !ok {
				return result, fmt.Errorf("instance type mismatch for %v", inst)
			}
			tfs.DataDir = d
			if s, err := renderSpec(t, tfs, inst.ID()+d); err == nil {
				result = append(result, s)
			}
		}
	default:
		s, err := renderSpec(t, inst, inst.ID())
		if err != nil {
			return result, fmt.Errorf("error rendering path for instance %v", inst)
		}
		result = append(result, s)
	}
	return result, nil
}

func renderSpec(t string, s interface{}, id string) (string, error) {
	tpl, err := template.New(id).Option("missingkey=error").Parse(t)
	if err != nil {
		return "", err
	}

	result := bytes.NewBufferString("")
	if err := tpl.Execute(result, s); err != nil {
		return "", err
	}
	return result.String(), nil
}
