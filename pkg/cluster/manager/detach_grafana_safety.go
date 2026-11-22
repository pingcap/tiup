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
	"net"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/set"
)

// grafanaDetachDir records one location — a directory occupied by a detached
// Grafana instance or still managed by a lifecycle operation, or a file the
// instance keeps there (Grafana's SQLite database). kind only labels the
// path in error messages ("deploy", "paths.data", "database.url", ...).
type grafanaDetachDir struct {
	cluster string
	node    string
	kind    string
	path    string
}

// grafanaDetachHostKey returns the key under which the same-host directory
// check groups paths. A host recorded as an IP literal is compared by
// address: the same IPv6 machine can be spelled compressed or expanded (and
// an IPv4-mapped literal duplicates its IPv4 form), and different spellings
// must not split one machine into two comparison groups, or the overlap
// check would silently skip it. A value that is not an IP literal keeps the
// recorded string: hostname aliases of one machine stay outside what a local
// check can resolve.
func grafanaDetachHostKey(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
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
// and destroying any of them runs the cleanup. Hosts are compared through
// grafanaDetachHostKey. Only locally recorded paths are compared — they must
// be absolute literals, since a relative path resolves against the remote
// login home, which the metadata does not record — and remote symlinks and
// mounts are invisible here, which is why detach still asks the operator to
// verify the actual layout.
func validateGrafanaDetachDirs(specManager *spec.SpecManager, name string, postDetach *spec.ClusterMeta, targets []spec.Instance) error {
	// Collect the directories the detached instances occupy, grouped by
	// manage host. These are the paths TiUP must never touch again.
	protected := make(map[string][]grafanaDetachDir)
	for _, instance := range targets {
		dirs, err := grafanaDetachInstanceDirs(name, postDetach, instance)
		if err != nil {
			return err
		}
		host := grafanaDetachHostKey(instance.GetManageHost())
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
				host := grafanaDetachHostKey(instance.GetManageHost())
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
				agentHosts.Insert(grafanaDetachHostKey(inst.GetManageHost()))
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

// grafanaDetachInstanceDirs returns the locations occupied by one instance:
// deploy, log and (possibly multiple) data directories, plus the
// component-specific overrides that live outside them, such as Grafana's
// paths.* settings and SQLite database location, or the TiFlash remote
// cache dir.
func grafanaDetachInstanceDirs(cluster string, metadata *spec.ClusterMeta, instance spec.Instance) ([]grafanaDetachDir, error) {
	id := instance.ID()
	// Every recorded path must be an absolute literal before anything else:
	// lifecycle cleanup passes the recorded values to an unquoted shell, a
	// relative value would resolve against the remote login home, which the
	// metadata does not record, and spec.Abs()/filepath.Clean() would
	// normalize away the whitespace and dot segments the shell still acts
	// on. Metadata saved by deploy and scale-out always satisfies this
	// (spec.ExpandRelativeDir runs before saving); in practice this only
	// rejects values introduced later through edit-config.
	for _, dir := range []struct{ kind, raw string }{
		{"deploy", instance.DeployDir()},
		{"log", instance.LogDir()},
	} {
		if dir.raw == "" { // nothing recorded, nothing for cleanup to delete
			continue
		}
		if err := grafanaDetachValidateRecordedPath(cluster, id, dir.kind, dir.raw); err != nil {
			return nil, err
		}
	}
	for raw := range strings.SplitSeq(instance.DataDir(), ",") {
		if raw == "" {
			continue
		}
		if err := grafanaDetachValidateRecordedPath(cluster, id, "data", raw); err != nil {
			return nil, err
		}
	}
	dirs := []grafanaDetachDir{
		{cluster, id, "deploy", spec.Abs(metadata.User, instance.DeployDir())},
		{cluster, id, "log", spec.Abs(metadata.User, instance.LogDir())},
	}
	for _, dir := range spec.MultiDirAbs(metadata.User, instance.DataDir()) {
		dirs = append(dirs, grafanaDetachDir{cluster, id, "data", dir})
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
				if err := grafanaDetachValidateRecordedPath(cluster, id, key, value); err != nil {
					return nil, err
				}
				dirs = append(dirs, grafanaDetachDir{cluster, id, key, value})
			}
		}
		// The saved dashboards live in Grafana's SQLite database, whose file
		// the [database] settings can move out of paths.data (covered above):
		// url accepts a bare or sqlite://-prefixed file location, path names
		// the sqlite file directly. Protect every local location that can be
		// determined from the metadata, so an overlap with a managed
		// directory is rejected instead of silently deleting the database
		// with that directory later.
		for _, key := range []string{"database.url", "database.path"} {
			value, found := grafana.Config[key]
			if !found {
				value, found = metadata.Topology.ServerConfigs.Grafana[key]
			}
			if !found || value == "" {
				// Unset or empty keeps Grafana's default location inside
				// paths.data, which the entry above (or the deploy directory
				// behind the template default) already covers.
				continue
			}
			location := value
			for _, prefix := range []string{"sqlite3://", "sqlite://"} {
				location = strings.TrimPrefix(location, prefix)
			}
			if strings.Contains(location, "://") {
				// mysql://, postgres:// and friends address an external
				// database server: there is no local file to protect.
				continue
			}
			if !filepath.IsAbs(location) {
				// A relative location resolves inside Grafana's data path or
				// working directory — both recorded above — unless it climbs
				// out with dot-dot segments, which can land anywhere.
				if grafanaDetachClimbsOut(location) {
					return nil, errors.Errorf("cannot determine cluster %q node %s %s path %q locally: a relative path with dot-dot segments can resolve outside Grafana's own directories; record an absolute literal path with `tiup cluster edit-config %s` before detaching", cluster, id, key, value, cluster)
				}
				continue
			}
			if err := grafanaDetachValidateRecordedPath(cluster, id, key, location); err != nil {
				return nil, err
			}
			dirs = append(dirs, grafanaDetachDir{cluster, id, key, location})
		}
	case spec.ComponentTiFlash:
		config := instance.(*spec.TiFlashInstance).InstanceSpec.(*spec.TiFlashSpec).Config
		if value, found := config[spec.TiFlashRemoteCacheDir]; found {
			dir, ok := value.(string)
			if !ok {
				return nil, errors.Errorf("cannot determine cluster %q node %s %s directory: the recorded value is not a plain string, which lifecycle cleanup would fail on; fix it with `tiup cluster edit-config %s` before detaching", cluster, id, spec.TiFlashRemoteCacheDir, cluster)
			}
			if err := grafanaDetachValidateRecordedPath(cluster, id, spec.TiFlashRemoteCacheDir, dir); err != nil {
				return nil, err
			}
			dirs = append(dirs, grafanaDetachDir{cluster, id, spec.TiFlashRemoteCacheDir, dir})
		}
	}
	for i := range dirs {
		dirs[i].path = filepath.Clean(dirs[i].path)
	}
	return dirs, nil
}

// grafanaDetachMonitoredDirs returns the deploy/data/log directories of the
// monitored components on one host; DestroyMonitored removes them once the
// host's last managed component is gone.
func grafanaDetachMonitoredDirs(cluster string, metadata *spec.ClusterMeta) ([]grafanaDetachDir, error) {
	options := metadata.Topology.MonitoredOptions
	// Same recorded-path rule as for instances: DestroyMonitored passes these
	// options to an unquoted shell, too.
	for _, dir := range []struct{ kind, raw string }{
		{"monitored deploy", options.DeployDir},
		{"monitored data", options.DataDir},
		{"monitored log", options.LogDir},
	} {
		if dir.raw == "" { // nothing recorded, nothing for cleanup to delete
			continue
		}
		if err := grafanaDetachValidateRecordedPath(cluster, "monitor", dir.kind, dir.raw); err != nil {
			return nil, err
		}
	}
	dirs := []grafanaDetachDir{
		{cluster, "monitor", "monitored deploy", options.DeployDir},
		{cluster, "monitor", "monitored data", options.DataDir},
		{cluster, "monitor", "monitored log", options.LogDir},
	}
	kept := make([]grafanaDetachDir, 0, len(dirs))
	for _, dir := range dirs {
		if dir.path == "" {
			continue
		}
		dir.path = filepath.Clean(dir.path)
		kept = append(kept, dir)
	}
	return kept, nil
}

// grafanaDetachValidateRecordedPath rejects recorded path values that this
// local check cannot map to one definite location: a relative value resolves
// against the remote login home, which the metadata does not record (an
// account provisioned outside TiUP can have any home, root's is /root), and
// whitespace or shell metacharacters reach lifecycle cleanup's unquoted
// shell. Metadata saved by deploy and scale-out is always absolute
// (spec.ExpandRelativeDir runs before saving), so in practice this only
// rejects values introduced later through edit-config.
func grafanaDetachValidateRecordedPath(cluster, node, kind, raw string) error {
	if !filepath.IsAbs(raw) {
		return errors.Errorf("cannot determine cluster %q node %s %s path %q locally: a relative path resolves against the remote login home, which this check cannot know; record an absolute literal path with `tiup cluster edit-config %s` before detaching", cluster, node, kind, raw, cluster)
	}
	if strings.ContainsAny(raw, "~$\x00\r\n\t *?[]`;&|<>\\\"'(){}!") {
		return errors.Errorf("cannot check cluster %q node %s %s path %q locally: whitespace or shell metacharacters in it would reach lifecycle cleanup's shell unquoted; record a plain literal path with `tiup cluster edit-config %s` before detaching", cluster, node, kind, raw, cluster)
	}
	return nil
}

// grafanaDetachClimbsOut reports whether a relative path contains a dot-dot
// segment and can therefore resolve above the directory it is relative to.
func grafanaDetachClimbsOut(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
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
