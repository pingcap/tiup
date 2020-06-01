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

package prepare

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/errutil"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

var (
	errNS                 = errorx.NewNamespace("check")
	errNSDeploy           = errNS.NewSubNamespace("deploy")
	errDeployDirConflict  = errNSDeploy.NewType("dir_conflict", errutil.ErrTraitPreCheck)
	errDeployPortConflict = errNSDeploy.NewType("port_conflict", errutil.ErrTraitPreCheck)
)

func fixDir(topo meta.Specification) func(string) string {
	return func(dir string) string {
		if dir != "" {
			return clusterutil.Abs(topo.GetGlobalOptions().User, dir)
		}
		return dir
	}
}

// CheckClusterDirConflict checks cluster dir conflict
func CheckClusterDirConflict(clusterName string, topo meta.Specification) error {
	type DirAccessor struct {
		dirKind  string
		accessor func(meta.Instance, meta.Specification) string
	}

	instanceDirAccessor := []DirAccessor{
		{dirKind: "deploy directory", accessor: func(instance meta.Instance, topo meta.Specification) string { return instance.DeployDir() }},
		{dirKind: "data directory", accessor: func(instance meta.Instance, topo meta.Specification) string { return instance.DataDir() }},
		{dirKind: "log directory", accessor: func(instance meta.Instance, topo meta.Specification) string { return instance.LogDir() }},
	}
	hostDirAccessor := []DirAccessor{
		{dirKind: "monitor deploy directory", accessor: func(instance meta.Instance, topo meta.Specification) string {
			return topo.GetMonitoredOptions().DeployDir
		}},
		{dirKind: "monitor data directory", accessor: func(instance meta.Instance, topo meta.Specification) string {
			return topo.GetMonitoredOptions().DataDir
		}},
		{dirKind: "monitor log directory", accessor: func(instance meta.Instance, topo meta.Specification) string {
			return topo.GetMonitoredOptions().LogDir
		}},
	}

	type Entry struct {
		clusterName string
		dirKind     string
		dir         string
		instance    meta.Instance
	}

	currentEntries := []Entry{}
	existingEntries := []Entry{}

	clusterDir := meta.ProfilePath(meta.TiOpsClusterDir)
	fileInfos, err := ioutil.ReadDir(clusterDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, fi := range fileInfos {
		if fi.Name() == clusterName {
			continue
		}

		if tiuputils.IsNotExist(meta.ClusterPath(fi.Name(), meta.MetaFileName)) {
			continue
		}
		metadata, err := meta.DMMetadata(fi.Name())
		if err != nil {
			return errors.Trace(err)
		}

		f := fixDir(metadata.Topology)
		metadata.Topology.IterInstance(func(inst meta.Instance) {
			for _, dirAccessor := range instanceDirAccessor {
				for _, dir := range strings.Split(f(dirAccessor.accessor(inst, metadata.Topology)), ",") {
					existingEntries = append(existingEntries, Entry{
						clusterName: fi.Name(),
						dirKind:     dirAccessor.dirKind,
						dir:         dir,
						instance:    inst,
					})
				}
			}
		})
		metadata.Topology.IterHost(func(inst meta.Instance) {
			for _, dirAccessor := range hostDirAccessor {
				for _, dir := range strings.Split(f(dirAccessor.accessor(inst, metadata.Topology)), ",") {
					existingEntries = append(existingEntries, Entry{
						clusterName: fi.Name(),
						dirKind:     dirAccessor.dirKind,
						dir:         dir,
						instance:    inst,
					})
				}
			}
		})
	}

	f := fixDir(topo)
	topo.IterInstance(func(inst meta.Instance) {
		for _, dirAccessor := range instanceDirAccessor {
			for _, dir := range strings.Split(f(dirAccessor.accessor(inst, topo)), ",") {
				currentEntries = append(currentEntries, Entry{
					dirKind:  dirAccessor.dirKind,
					dir:      dir,
					instance: inst,
				})
			}
		}
	})
	topo.IterHost(func(inst meta.Instance) {
		for _, dirAccessor := range hostDirAccessor {
			for _, dir := range strings.Split(f(dirAccessor.accessor(inst, topo)), ",") {
				currentEntries = append(currentEntries, Entry{
					dirKind:  dirAccessor.dirKind,
					dir:      dir,
					instance: inst,
				})
			}
		}
	})

	for _, d1 := range currentEntries {
		// data_dir is relative to deploy_dir by default, so they can be with
		// same (sub) paths as long as the deploy_dirs are different
		if d1.dirKind == "data directory" && !strings.HasPrefix(d1.dir, "/") {
			continue
		}
		for _, d2 := range existingEntries {
			if d1.instance.GetHost() != d2.instance.GetHost() {
				continue
			}

			if d1.dir == d2.dir && d1.dir != "" {
				properties := map[string]string{
					"ThisDirKind":    d1.dirKind,
					"ThisDir":        d1.dir,
					"ThisComponent":  d1.instance.ComponentName(),
					"ThisHost":       d1.instance.GetHost(),
					"ExistCluster":   d2.clusterName,
					"ExistDirKind":   d2.dirKind,
					"ExistDir":       d2.dir,
					"ExistComponent": d2.instance.ComponentName(),
					"ExistHost":      d2.instance.GetHost(),
				}
				zap.L().Info("Meet deploy directory conflict", zap.Any("info", properties))
				return errDeployDirConflict.New("Deploy directory conflicts to an existing cluster").WithProperty(cliutil.SuggestionFromTemplate(`
The directory you specified in the topology file is:
  Directory: {{ColorKeyword}}{{.ThisDirKind}} {{.ThisDir}}{{ColorReset}}
  Component: {{ColorKeyword}}{{.ThisComponent}} {{.ThisHost}}{{ColorReset}}

It conflicts to a directory in the existing cluster:
  Existing Cluster Name: {{ColorKeyword}}{{.ExistCluster}}{{ColorReset}}
  Existing Directory:    {{ColorKeyword}}{{.ExistDirKind}} {{.ExistDir}}{{ColorReset}}
  Existing Component:    {{ColorKeyword}}{{.ExistComponent}} {{.ExistHost}}{{ColorReset}}

Please change to use another directory or another host.
`, properties))
			}
		}
	}

	return nil
}

// CheckClusterPortConflict checks cluster dir conflict
func CheckClusterPortConflict(clusterName string, topo meta.Specification) error {
	clusterDir := meta.ProfilePath(meta.TiOpsClusterDir)
	fileInfos, err := ioutil.ReadDir(clusterDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	type Entry struct {
		clusterName string
		instance    meta.Instance
		port        int
	}

	currentEntries := []Entry{}
	existingEntries := []Entry{}

	for _, fi := range fileInfos {
		if fi.Name() == clusterName {
			continue
		}

		if tiuputils.IsNotExist(meta.ClusterPath(fi.Name(), meta.MetaFileName)) {
			continue
		}
		metadata, err := meta.ClusterMetadata(fi.Name())
		if err != nil {
			return errors.Trace(err)
		}

		metadata.Topology.IterInstance(func(inst meta.Instance) {
			for _, port := range inst.UsedPorts() {
				existingEntries = append(existingEntries, Entry{
					clusterName: fi.Name(),
					instance:    inst,
					port:        port,
				})
			}
		})
	}

	topo.IterInstance(func(inst meta.Instance) {
		for _, port := range inst.UsedPorts() {
			currentEntries = append(currentEntries, Entry{
				instance: inst,
				port:     port,
			})
		}
	})

	for _, p1 := range currentEntries {
		for _, p2 := range existingEntries {
			if p1.instance.GetHost() != p2.instance.GetHost() {
				continue
			}

			if p1.port == p2.port {
				properties := map[string]string{
					"ThisPort":       strconv.Itoa(p1.port),
					"ThisComponent":  p1.instance.ComponentName(),
					"ThisHost":       p1.instance.GetHost(),
					"ExistCluster":   p2.clusterName,
					"ExistPort":      strconv.Itoa(p2.port),
					"ExistComponent": p2.instance.ComponentName(),
					"ExistHost":      p2.instance.GetHost(),
				}
				zap.L().Info("Meet deploy port conflict", zap.Any("info", properties))
				return errDeployPortConflict.New("Deploy port conflicts to an existing cluster").WithProperty(cliutil.SuggestionFromTemplate(`
The port you specified in the topology file is:
  Port:      {{ColorKeyword}}{{.ThisPort}}{{ColorReset}}
  Component: {{ColorKeyword}}{{.ThisComponent}} {{.ThisHost}}{{ColorReset}}

It conflicts to a port in the existing cluster:
  Existing Cluster Name: {{ColorKeyword}}{{.ExistCluster}}{{ColorReset}}
  Existing Port:         {{ColorKeyword}}{{.ExistPort}}{{ColorReset}}
  Existing Component:    {{ColorKeyword}}{{.ExistComponent}} {{.ExistHost}}{{ColorReset}}

Please change to use another port or another host.
`, properties))
			}
		}
	}

	return nil
}

// BuildDownloadCompTasks build download component tasks
func BuildDownloadCompTasks(version string, topo meta.Specification) []*task.StepDisplay {
	var tasks []*task.StepDisplay
	uniqueTaskList := make(map[string]struct{}) // map["comp-os-arch"]{}
	topo.IterInstance(func(inst meta.Instance) {
		key := fmt.Sprintf("%s-%s-%s", inst.ComponentName(), inst.OS(), inst.Arch())
		if _, found := uniqueTaskList[key]; !found {
			uniqueTaskList[key] = struct{}{}

			version := meta.ComponentVersion(inst.ComponentName(), version)
			t := task.NewBuilder().
				Download(inst.ComponentName(), inst.OS(), inst.Arch(), version).
				BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)",
					inst.ComponentName(), version, inst.OS(), inst.Arch()))
			tasks = append(tasks, t)
		}
	})
	return tasks
}
