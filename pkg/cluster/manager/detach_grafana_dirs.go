// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
)

type grafanaDetachDir struct {
	cluster string
	node    string
	kind    string
	path    string
}

func (m *Manager) validateGrafanaDetachDirs(name string, remaining *spec.ClusterMeta, targets []spec.Instance) error {
	protected := make(map[string][]grafanaDetachDir)
	for _, instance := range targets {
		dirs, err := grafanaDetachInstanceDirs(name, remaining, instance)
		if err != nil {
			return err
		}
		host := instance.GetManageHost()
		protected[host] = append(protected[host], dirs...)
	}
	names, err := m.specManager.List()
	if err != nil {
		return err
	}
	for _, cluster := range names {
		metadata := remaining
		if cluster != name {
			metadata = &spec.ClusterMeta{}
			if err := m.specManager.Metadata(cluster, metadata); err != nil {
				return errors.Annotatef(err, "cannot check directories recorded for cluster %q in this profile", cluster)
			}
		}
		if metadata.Topology == nil {
			return errors.Errorf("cannot check directories: cluster %q has no topology in its metadata", cluster)
		}
		managed := make(map[string][]grafanaDetachDir)
		for _, component := range metadata.Topology.ComponentsByStartOrder() {
			for _, instance := range component.Instances() {
				host := instance.GetManageHost()
				if len(protected[host]) == 0 {
					continue
				}
				dirs, err := grafanaDetachInstanceDirs(cluster, metadata, instance)
				if err != nil {
					return err
				}
				managed[host] = append(managed[host], dirs...)
			}
		}
		hosts, noAgentHosts := getMonitorHosts(metadata.Topology)
		options := metadata.Topology.MonitoredOptions
		for host := range hosts {
			if len(protected[host]) == 0 || noAgentHosts.Exist(host) {
				continue
			}
			deploy := spec.Abs(metadata.Topology.GlobalOptions.User, options.DeployDir)
			data := options.DataDir
			if data != "" && !filepath.IsAbs(data) {
				data = filepath.Join(deploy, data)
			}
			for _, dir := range []grafanaDetachDir{
				{cluster, "monitor", "monitored deploy", deploy},
				{cluster, "monitor", "monitored data", data},
				{cluster, "monitor", "monitored log", spec.Abs(metadata.Topology.GlobalOptions.User, options.LogDir)},
			} {
				if dir.path != "" {
					managed[host] = append(managed[host], dir)
				}
			}
			// Deployment resolves relative monitored data below its deploy dir,
			// but DestroyMonitored passes it directly to a shell in the SSH home.
			// Cover both recorded interpretations without changing that cleanup.
			if options.DataDir != "" && !filepath.IsAbs(options.DataDir) {
				managed[host] = append(managed[host], grafanaDetachDir{
					cluster, "monitor", "monitored data cleanup", spec.Abs(metadata.User, options.DataDir),
				})
			}
		}
		for host, dirs := range managed {
			for _, dir := range dirs {
				if err := dir.validate(); err != nil {
					return err
				}
				for _, target := range protected[host] {
					if grafanaDetachPathsOverlap(target.path, dir.path) {
						return errors.Errorf("cannot detach Grafana %s: %s %q overlaps cluster %q node %s %s %q on host %s", target.node, target.kind, target.path, dir.cluster, dir.node, dir.kind, dir.path, host)
					}
				}
			}
		}
	}
	return nil
}

func grafanaDetachInstanceDirs(cluster string, metadata *spec.ClusterMeta, instance spec.Instance) ([]grafanaDetachDir, error) {
	dirs := []grafanaDetachDir{
		{cluster, instance.ID(), "deploy", spec.Abs(metadata.User, instance.DeployDir())},
		{cluster, instance.ID(), "log", spec.Abs(metadata.User, instance.LogDir())},
	}
	for _, dir := range spec.MultiDirAbs(metadata.User, instance.DataDir()) {
		dirs = append(dirs, grafanaDetachDir{cluster, instance.ID(), "data", dir})
	}
	switch instance.ComponentName() {
	case spec.ComponentGrafana:
		grafana := instance.(*spec.GrafanaInstance).InstanceSpec.(*spec.GrafanaSpec)
		for _, key := range []string{"paths.data", "paths.logs", "paths.plugins", "paths.provisioning"} {
			value, found := grafana.Config[key]
			if !found {
				value, found = metadata.Topology.ServerConfigs.Grafana[key]
			}
			if found {
				dirs = append(dirs, grafanaDetachDir{cluster, instance.ID(), key, value})
			}
		}
	case spec.ComponentTiFlash:
		config := instance.(*spec.TiFlashInstance).InstanceSpec.(*spec.TiFlashSpec).Config
		if value, found := config[spec.TiFlashRemoteCacheDir]; found {
			dir, ok := value.(string)
			if !ok {
				return nil, errors.Errorf("cannot determine cluster %q node %s %s directory", cluster, instance.ID(), spec.TiFlashRemoteCacheDir)
			}
			dirs = append(dirs, grafanaDetachDir{cluster, instance.ID(), spec.TiFlashRemoteCacheDir, dir})
		}
	}
	for i := range dirs {
		if err := dirs[i].validate(); err != nil {
			return nil, err
		}
		dirs[i].path = filepath.Clean(dirs[i].path)
	}
	return dirs, nil
}

func (d grafanaDetachDir) validate() error {
	// Grafana expands providers and relative paths on the remote host. Resolving
	// them (or symlinks) on the control machine would give a false safety check.
	// Existing lifecycle cleanup passes paths to a shell without quoting. A glob
	// or shell expression cannot be checked by literal directory comparison.
	if !filepath.IsAbs(d.path) || strings.ContainsAny(d.path, "$\x00\r\n\t *?[]`;&|<>\\\"'(){}!") {
		return errors.Errorf("cannot determine cluster %q node %s %s directory %q locally; record an absolute literal path matching the actual configuration before detaching", d.cluster, d.node, d.kind, d.path)
	}
	return nil
}

func grafanaDetachPathsOverlap(first, second string) bool {
	first, second = filepath.Clean(first), filepath.Clean(second)
	return first == second ||
		strings.HasPrefix(first, strings.TrimRight(second, string(filepath.Separator))+string(filepath.Separator)) ||
		strings.HasPrefix(second, strings.TrimRight(first, string(filepath.Separator))+string(filepath.Separator))
}
