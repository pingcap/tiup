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
	"context"
	"fmt"
	"html/template"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/set"
	"go.uber.org/zap"
)

// TransferOptions for exec shell commanm.
type TransferOptions struct {
	Local    string
	Remote   string
	Pull     bool // default to push
	Limit    int  // rate limit in Kbit/s
	Compress bool // enable compress
}

// Transfer copies files from or to host in the tidb cluster.
func (m *Manager) Transfer(name string, opt TransferOptions, gOpt operator.Options) error {
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

	uniqueHosts := map[string]set.StringSet{} // host-sshPort -> {remote-path}
	topo.IterInstance(func(inst spec.Instance) {
		key := fmt.Sprintf("%d-%s", inst.GetSSHPort(), inst.GetManageHost())
		if _, found := uniqueHosts[key]; !found {
			if len(gOpt.Roles) > 0 && !filterRoles.Exist(inst.Role()) {
				return
			}

			if len(gOpt.Nodes) > 0 && (!filterNodes.Exist(inst.GetHost()) || !filterNodes.Exist(inst.GetManageHost())) {
				return
			}

			// render remote path
			instPath := opt.Remote
			paths, err := renderInstanceSpec(instPath, inst)
			if err != nil {
				m.logger.Debugf("error rendering remote path with spec: %s", err)
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
		host := hostKey[len(strings.Split(hostKey, "-")[0])+1:]
		for _, p := range i.Slice() {
			t := task.NewBuilder(m.logger)
			if opt.Pull {
				t.CopyFile(p, srcPath, host, opt.Pull, opt.Limit, opt.Compress)
			} else {
				t.CopyFile(srcPath, p, host, opt.Pull, opt.Limit, opt.Compress)
			}
			shellTasks = append(shellTasks, t.Build())
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

	return nil
}

func renderInstanceSpec(t string, inst spec.Instance) ([]string, error) {
	result := make([]string, 0)
	switch inst.ComponentName() {
	case spec.ComponentTiFlash:
		for _, d := range strings.Split(inst.DataDir(), ",") {
			tfs, ok := inst.(*spec.TiFlashInstance).InstanceSpec.(*spec.TiFlashSpec)
			if !ok {
				return result, perrs.Errorf("instance type mismatch for %v", inst)
			}
			tfs.DataDir = d
			key := inst.ID() + d + uuid.New().String()
			s, err := renderSpec(t, inst.(*spec.TiFlashInstance), key)
			if err != nil {
				zap.L().Debug("error rendering tiflash spec", zap.Error(err))
			}
			result = append(result, s)
		}
	default:
		s, err := renderSpec(t, inst, inst.ID())
		if err != nil {
			return result, perrs.Errorf("error rendering path for instance %v", inst)
		}
		result = append(result, s)
	}
	return result, nil
}

func renderSpec(t string, s any, id string) (string, error) {
	// Only apply on *spec.TiDBInstance and *spec.PDInstance etc.
	if v := reflect.ValueOf(s); v.Kind() == reflect.Ptr {
		if v = v.Elem(); !v.IsValid() {
			return "", perrs.Errorf("invalid spec")
		}
		if v = v.FieldByName("BaseInstance"); !v.IsValid() {
			return "", perrs.Errorf("field BaseInstance not found")
		}
		if v = v.FieldByName("InstanceSpec"); !v.IsValid() {
			return "", perrs.Errorf("field InstanceSpec not found")
		}
		s = v.Interface()
	}

	tpl, err := template.New(id).Option("missingkey=error").Parse(t)
	if err != nil {
		return "", err
	}

	result := bytes.NewBufferString("")
	if err := tpl.Execute(result, s); err != nil {
		zap.L().Debug("missing key when parsing: %s", zap.Error(err))
		return "", err
	}
	return result.String(), nil
}
