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
	"github.com/pingcap/tiup/pkg/set"
)

// grafanaDetachDir records one directory either occupied by a detached Grafana
// instance or still managed by a lifecycle operation. kind only labels the
// path in error messages ("deploy", "paths.data", "monitored data", ...).
type grafanaDetachDir struct {
	cluster string
	node    string
	kind    string
	path    string
}

// validateGrafanaDetachDirs rejects a detach whose Grafana directories overlap
// any directory TiUP still manages.
//
// Detach promises that remote files are left untouched, but later lifecycle
// operations keep cleaning up recorded directories through a shell: destroy
// and scale-in remove deploy/data/log directories, reload and upgrade rewrite
// deploy directories, and a monitored host is cleaned up once its last managed
// component is gone. A directory shared between a detached Grafana and any
// managed instance would eventually be deleted or overwritten, breaking that
// promise. The check therefore covers every cluster in this profile, not just
// the one being detached: the same host can be managed by several clusters,
// and destroying any of them runs the cleanup. Only locally recorded paths
// are compared; remote symlinks and mounts are invisible here, which is why
// detach still asks the operator to verify the actual layout.
func validateGrafanaDetachDirs(specManager *spec.SpecManager, name string, postDetach *spec.ClusterMeta, targets []spec.Instance) error {
	// Collect the directories the detached instances occupy, grouped by
	// manage host. These are the paths TiUP must never touch again.
	protected := make(map[string][]grafanaDetachDir)
	for _, instance := range targets {
		dirs, err := grafanaDetachInstanceDirs(name, postDetach, instance)
		if err != nil {
			return err
		}
		host := instance.GetManageHost()
		protected[host] = append(protected[host], dirs...)
	}
	// Collect the directories TiUP may still clean up or rewrite on those
	// hosts, from every cluster in the profile. The detached cluster
	// contributes the post-detach topology passed in; the others are loaded
	// from disk.
	names, err := specManager.List()
	if err != nil {
		return err
	}
	for _, cluster := range names {
		metadata := postDetach
		if cluster != name {
			metadata = &spec.ClusterMeta{}
			if err := specManager.Metadata(cluster, metadata); err != nil {
				return errors.Annotatef(err, "cannot check directories recorded for cluster %q in this profile", cluster)
			}
		}
		if metadata.Topology == nil {
			return errors.Errorf("cannot check directories: cluster %q has no topology in its metadata", cluster)
		}
		// Instance directories of every component deployed on the affected
		// hosts (including Grafana path overrides and TiFlash remote caches
		// via grafanaDetachInstanceDirs).
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
		// Monitored (node_exporter / blackbox_exporter) directories on the
		// affected hosts; they belong to no instance. DestroyMonitored runs
		// whenever the last instance destroyed on a manage host does not
		// ignore exporters, so one non-ignoring instance keeps the host's
		// monitored directories in play; do not skip the host just because
		// some other instance on it ignores them.
		agentHosts := set.NewStringSet()
		metadata.Topology.IterInstance(func(inst spec.Instance) {
			if !inst.IgnoreMonitorAgent() {
				agentHosts.Insert(inst.GetManageHost())
			}
		})
		for host := range agentHosts {
			if len(protected[host]) == 0 {
				continue
			}
			dirs, err := grafanaDetachMonitoredDirs(cluster, metadata)
			if err != nil {
				return err
			}
			managed[host] = append(managed[host], dirs...)
		}
		// Reject if any protected directory equals, contains, or is contained
		// in a managed directory on the same host.
		for host, dirs := range managed {
			for _, dir := range dirs {
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

// grafanaDetachInstanceDirs returns the directories occupied by one instance:
// deploy, log and (possibly multiple) data directories, plus the
// component-specific overrides that live outside them, such as Grafana's
// paths.* settings or the TiFlash remote cache dir.
func grafanaDetachInstanceDirs(cluster string, metadata *spec.ClusterMeta, instance spec.Instance) ([]grafanaDetachDir, error) {
	// Lifecycle cleanup passes the raw recorded values to an unquoted shell;
	// check them before spec.Abs()/filepath.Clean() normalizes away the
	// whitespace and dot segments the remote shell would still act on.
	for _, dir := range []struct{ kind, raw string }{
		{"deploy", instance.DeployDir()},
		{"log", instance.LogDir()},
	} {
		if err := grafanaDetachValidateRecordedPath(cluster, instance.ID(), dir.kind, dir.raw); err != nil {
			return nil, err
		}
	}
	for _, raw := range strings.Split(instance.DataDir(), ",") {
		if err := grafanaDetachValidateRecordedPath(cluster, instance.ID(), "data", raw); err != nil {
			return nil, err
		}
	}
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

// grafanaDetachMonitoredDirs returns the deploy/data/log directories of the
// monitored components on one host; DestroyMonitored removes them once the
// host's last managed component is gone. A relative data dir is listed twice
// on purpose: deployment resolves it below the monitored deploy dir, but
// DestroyMonitored passes it to a shell directly, i.e. relative to the SSH
// home. Cover both recorded interpretations without changing that cleanup.
func grafanaDetachMonitoredDirs(cluster string, metadata *spec.ClusterMeta) ([]grafanaDetachDir, error) {
	options := metadata.Topology.MonitoredOptions
	// Same raw-value check as for instances: DestroyMonitored passes these
	// options to an unquoted shell, too.
	for _, dir := range []struct{ kind, raw string }{
		{"monitored deploy", options.DeployDir},
		{"monitored data", options.DataDir},
		{"monitored log", options.LogDir},
	} {
		if err := grafanaDetachValidateRecordedPath(cluster, "monitor", dir.kind, dir.raw); err != nil {
			return nil, err
		}
	}
	deploy := spec.Abs(metadata.Topology.GlobalOptions.User, options.DeployDir)
	data := options.DataDir
	if data != "" && !filepath.IsAbs(data) {
		data = filepath.Join(deploy, data)
	}
	dirs := []grafanaDetachDir{
		{cluster, "monitor", "monitored deploy", deploy},
		{cluster, "monitor", "monitored data", data},
		{cluster, "monitor", "monitored log", spec.Abs(metadata.Topology.GlobalOptions.User, options.LogDir)},
	}
	if options.DataDir != "" && !filepath.IsAbs(options.DataDir) {
		dirs = append(dirs, grafanaDetachDir{
			cluster, "monitor", "monitored data cleanup", spec.Abs(metadata.User, options.DataDir),
		})
	}
	var kept []grafanaDetachDir
	for _, dir := range dirs {
		if dir.path == "" {
			continue
		}
		if err := dir.validate(); err != nil {
			return nil, err
		}
		dir.path = filepath.Clean(dir.path)
		kept = append(kept, dir)
	}
	return kept, nil
}

// grafanaDetachValidateRecordedPath rejects raw recorded path values that the
// remote shell would interpret differently than this local check does.
// spec.Abs trims surrounding whitespace and filepath.Clean drops segments
// such as "grafana /../", but lifecycle cleanup passes the raw value to an
// unquoted `rm -rf`, where the same string word-splits into several paths or
// expands globs. Raw values may remain relative: the remote shell resolves
// them against the SSH home, matching spec.Abs.
func grafanaDetachValidateRecordedPath(cluster, node, kind, raw string) error {
	if strings.ContainsAny(raw, "~$\x00\r\n\t *?[]`;&|<>\\\"'(){}!") {
		return errors.Errorf("cannot check cluster %q node %s %s path %q locally: whitespace or shell metacharacters in it would reach lifecycle cleanup's shell unquoted; record plain literal paths before detaching", cluster, node, kind, raw)
	}
	return nil
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

// grafanaDetachPathsOverlap reports whether first and second denote the same
// directory or one of them contains the other. Both directions matter: a
// managed directory inside the detached one will be removed by a future
// scale-in, and a detached directory inside a managed one is removed with it
// by a future destroy or scale-in of that instance.
func grafanaDetachPathsOverlap(first, second string) bool {
	first, second = filepath.Clean(first), filepath.Clean(second)
	return first == second ||
		strings.HasPrefix(first, strings.TrimRight(second, string(filepath.Separator))+string(filepath.Separator)) ||
		strings.HasPrefix(second, strings.TrimRight(first, string(filepath.Separator))+string(filepath.Separator))
}
