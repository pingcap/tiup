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
	"strconv"
	"strings"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/errutil"
	"go.uber.org/zap"
)

var (
	errNS                 = errorx.NewNamespace("check")
	errNSDeploy           = errNS.NewSubNamespace("deploy")
	errDeployDirConflict  = errNSDeploy.NewType("dir_conflict", errutil.ErrTraitPreCheck)
	errDeployPortConflict = errNSDeploy.NewType("port_conflict", errutil.ErrTraitPreCheck)
)

func fixDir(topo spec.Topology) func(string) string {
	return func(dir string) string {
		if dir != "" {
			return clusterutil.Abs(topo.BaseTopo().GlobalOptions.User, dir)
		}
		return dir
	}
}

// CheckClusterDirConflict checks cluster dir conflict
func CheckClusterDirConflict(clusterSpec *spec.SpecManager, clusterName string, topo spec.Topology) error {
	type DirAccessor struct {
		dirKind  string
		accessor func(spec.Instance, spec.Topology) string
	}

	instanceDirAccessor := []DirAccessor{
		{dirKind: "deploy directory", accessor: func(instance spec.Instance, topo spec.Topology) string { return instance.DeployDir() }},
		{dirKind: "data directory", accessor: func(instance spec.Instance, topo spec.Topology) string { return instance.DataDir() }},
		{dirKind: "log directory", accessor: func(instance spec.Instance, topo spec.Topology) string { return instance.LogDir() }},
	}
	hostDirAccessor := []DirAccessor{
		{dirKind: "monitor deploy directory", accessor: func(instance spec.Instance, topo spec.Topology) string {
			m := topo.BaseTopo().MonitoredOptions
			if m == nil {
				return ""
			}
			return topo.BaseTopo().MonitoredOptions.DeployDir
		}},
		{dirKind: "monitor data directory", accessor: func(instance spec.Instance, topo spec.Topology) string {
			m := topo.BaseTopo().MonitoredOptions
			if m == nil {
				return ""
			}
			return topo.BaseTopo().MonitoredOptions.DataDir
		}},
		{dirKind: "monitor log directory", accessor: func(instance spec.Instance, topo spec.Topology) string {
			m := topo.BaseTopo().MonitoredOptions
			if m == nil {
				return ""
			}
			return topo.BaseTopo().MonitoredOptions.LogDir
		}},
	}

	type Entry struct {
		clusterName string
		dirKind     string
		dir         string
		instance    spec.Instance
	}

	currentEntries := []Entry{}
	existingEntries := []Entry{}

	names, err := clusterSpec.List()
	if err != nil {
		return errors.AddStack(err)
	}

	for _, name := range names {
		if name == clusterName {
			continue
		}

		metadata := clusterSpec.NewMetadata()
		err := clusterSpec.Metadata(name, metadata)
		if err != nil {
			return errors.Trace(err)
		}

		topo := metadata.GetTopology()

		f := fixDir(topo)
		topo.IterInstance(func(inst spec.Instance) {
			for _, dirAccessor := range instanceDirAccessor {
				for _, dir := range strings.Split(f(dirAccessor.accessor(inst, topo)), ",") {
					existingEntries = append(existingEntries, Entry{
						clusterName: name,
						dirKind:     dirAccessor.dirKind,
						dir:         dir,
						instance:    inst,
					})
				}
			}
		})
		spec.IterHost(topo, func(inst spec.Instance) {
			for _, dirAccessor := range hostDirAccessor {
				for _, dir := range strings.Split(f(dirAccessor.accessor(inst, topo)), ",") {
					existingEntries = append(existingEntries, Entry{
						clusterName: name,
						dirKind:     dirAccessor.dirKind,
						dir:         dir,
						instance:    inst,
					})
				}
			}
		})
	}

	f := fixDir(topo)
	topo.IterInstance(func(inst spec.Instance) {
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

	spec.IterHost(topo, func(inst spec.Instance) {
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
func CheckClusterPortConflict(clusterSpec *spec.SpecManager, clusterName string, topo spec.Topology) error {
	type Entry struct {
		clusterName string
		instance    spec.Instance
		port        int
	}

	currentEntries := []Entry{}
	existingEntries := []Entry{}

	names, err := clusterSpec.List()
	if err != nil {
		return errors.AddStack(err)
	}

	for _, name := range names {
		if name == clusterName {
			continue
		}

		metadata := clusterSpec.NewMetadata()
		err := clusterSpec.Metadata(name, metadata)
		if err != nil {
			return errors.Trace(err)
		}

		metadata.GetTopology().IterInstance(func(inst spec.Instance) {
			for _, port := range inst.UsedPorts() {
				existingEntries = append(existingEntries, Entry{
					clusterName: name,
					instance:    inst,
					port:        port,
				})
			}
		})
	}

	topo.IterInstance(func(inst spec.Instance) {
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

// InstanceIter to iterate instance.
type InstanceIter interface {
	IterInstance(func(inst spec.Instance))
}

// BuildDownloadCompTasks build download component tasks
func BuildDownloadCompTasks(version string, instanceIter InstanceIter) []*task.StepDisplay {
	var tasks []*task.StepDisplay
	uniqueTaskList := make(map[string]struct{}) // map["comp-os-arch"]{}
	instanceIter.IterInstance(func(inst spec.Instance) {
		key := fmt.Sprintf("%s-%s-%s", inst.ComponentName(), inst.OS(), inst.Arch())
		if _, found := uniqueTaskList[key]; !found {
			uniqueTaskList[key] = struct{}{}

			// download spark as dependency of tispark
			if inst.ComponentName() == spec.ComponentTiSpark {
				tasks = append(tasks, buildDownloadSparkTask(version, inst))
			}

			version := spec.ComponentVersion(inst.ComponentName(), version)
			t := task.NewBuilder().
				Download(inst.ComponentName(), inst.OS(), inst.Arch(), version).
				BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)",
					inst.ComponentName(), version, inst.OS(), inst.Arch()))
			tasks = append(tasks, t)
		}
	})
	return tasks
}

// buildDownloadSparkTask build download task for spark, which is a dependency of tispark
// FIXME: this is a hack and should be replaced by dependency handling in manifest processing
func buildDownloadSparkTask(version string, inst spec.Instance) *task.StepDisplay {
	ver := spec.ComponentVersion(spec.ComponentSpark, version)
	return task.NewBuilder().
		Download(spec.ComponentSpark, inst.OS(), inst.Arch(), ver).
		BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)",
			spec.ComponentSpark, version, inst.OS(), inst.Arch()))
}
