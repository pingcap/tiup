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

package spec

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/api"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

// pre defined error types
var (
	errNSDeploy              = errNS.NewSubNamespace("deploy")
	errDeployDirConflict     = errNSDeploy.NewType("dir_conflict", utils.ErrTraitPreCheck)
	errDeployDirOverlap      = errNSDeploy.NewType("dir_overlap", utils.ErrTraitPreCheck)
	errDeployPortConflict    = errNSDeploy.NewType("port_conflict", utils.ErrTraitPreCheck)
	ErrNoTiSparkMaster       = errors.New("there must be a Spark master node if you want to use the TiSpark component")
	ErrMultipleTiSparkMaster = errors.New("a TiSpark enabled cluster with more than 1 Spark master node is not supported")
	ErrMultipleTisparkWorker = errors.New("multiple TiSpark workers on the same host is not supported by Spark")
	ErrUserOrGroupInvalid    = errors.New(`linux username and groupname must start with a lower case letter or an underscore, ` +
		`followed by lower case letters, digits, underscores, or dashes. ` +
		`Usernames may only be up to 32 characters long. ` +
		`Groupnames may only be up to 16 characters long.`)
)

// Linux username and groupname must start with a lower case letter or an underscore,
// followed by lower case letters, digits, underscores, or dashes.
// ref https://man7.org/linux/man-pages/man8/useradd.8.html
// ref https://man7.org/linux/man-pages/man8/groupadd.8.html
var (
	reUser  = regexp.MustCompile(`^[a-z_]([a-z0-9_-]{0,31}|[a-z0-9_-]{0,30}\$)$`)
	reGroup = regexp.MustCompile(`^[a-z_]([a-z0-9_-]{0,15})$`)
)

func fixDir(topo Topology) func(string) string {
	return func(dir string) string {
		if dir != "" {
			return Abs(topo.BaseTopo().GlobalOptions.User, dir)
		}
		return dir
	}
}

// DirAccessor stands for a directory accessor for an instance
type DirAccessor struct {
	dirKind  string
	accessor func(Instance, Topology) string
}

func dirAccessors() ([]DirAccessor, []DirAccessor) {
	instanceDirAccessor := []DirAccessor{
		{dirKind: "deploy directory", accessor: func(instance Instance, topo Topology) string { return instance.DeployDir() }},
		{dirKind: "data directory", accessor: func(instance Instance, topo Topology) string { return instance.DataDir() }},
		{dirKind: "log directory", accessor: func(instance Instance, topo Topology) string { return instance.LogDir() }},
	}
	hostDirAccessor := []DirAccessor{
		{dirKind: "monitor deploy directory", accessor: func(instance Instance, topo Topology) string {
			m := topo.BaseTopo().MonitoredOptions
			if m == nil {
				return ""
			}
			return m.DeployDir
		}},
		{dirKind: "monitor data directory", accessor: func(instance Instance, topo Topology) string {
			m := topo.BaseTopo().MonitoredOptions
			if m == nil {
				return ""
			}
			return m.DataDir
		}},
		{dirKind: "monitor log directory", accessor: func(instance Instance, topo Topology) string {
			m := topo.BaseTopo().MonitoredOptions
			if m == nil {
				return ""
			}
			return m.LogDir
		}},
	}

	return instanceDirAccessor, hostDirAccessor
}

// DirEntry stands for a directory with attributes and instance
type DirEntry struct {
	clusterName string
	dirKind     string
	dir         string
	instance    Instance
}

func appendEntries(name string, topo Topology, inst Instance, dirAccessor DirAccessor, targets []DirEntry) []DirEntry {
	for _, dir := range strings.Split(fixDir(topo)(dirAccessor.accessor(inst, topo)), ",") {
		targets = append(targets, DirEntry{
			clusterName: name,
			dirKind:     dirAccessor.dirKind,
			dir:         dir,
			instance:    inst,
		})
	}

	return targets
}

// CheckClusterDirConflict checks cluster dir conflict or overlap
func CheckClusterDirConflict(clusterList map[string]Metadata, clusterName string, topo Topology) error {
	instanceDirAccessor, hostDirAccessor := dirAccessors()
	currentEntries := []DirEntry{}
	existingEntries := []DirEntry{}

	// rebuild existing disk status
	for name, metadata := range clusterList {
		if name == clusterName {
			continue
		}

		topo := metadata.GetTopology()

		topo.IterInstance(func(inst Instance) {
			for _, dirAccessor := range instanceDirAccessor {
				existingEntries = appendEntries(name, topo, inst, dirAccessor, existingEntries)
			}
		})
		IterHost(topo, func(inst Instance) {
			for _, dirAccessor := range hostDirAccessor {
				existingEntries = appendEntries(name, topo, inst, dirAccessor, existingEntries)
			}
		})
	}

	topo.IterInstance(func(inst Instance) {
		for _, dirAccessor := range instanceDirAccessor {
			currentEntries = appendEntries(clusterName, topo, inst, dirAccessor, currentEntries)
		}
	})
	IterHost(topo, func(inst Instance) {
		for _, dirAccessor := range hostDirAccessor {
			currentEntries = appendEntries(clusterName, topo, inst, dirAccessor, currentEntries)
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

			// ignore conflict in the case when both sides are monitor and either one of them
			// is marked as ignore exporter.
			if strings.HasPrefix(d1.dirKind, "monitor") &&
				strings.HasPrefix(d2.dirKind, "monitor") &&
				(d1.instance.IgnoreMonitorAgent() || d2.instance.IgnoreMonitorAgent()) {
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
				return errDeployDirConflict.New("Deploy directory conflicts to an existing cluster").WithProperty(tui.SuggestionFromTemplate(`
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

	return CheckClusterDirOverlap(currentEntries)
}

// CheckClusterDirOverlap checks cluster dir overlaps with data or log.
// this should only be used across clusters.
// we don't allow to deploy log under data, and vise versa.
// ref https://github.com/pingcap/tiup/issues/1047#issuecomment-761711508
func CheckClusterDirOverlap(entries []DirEntry) error {
	ignore := func(d1, d2 DirEntry) bool {
		return (d1.instance.GetHost() != d2.instance.GetHost()) ||
			d1.dir == "" || d2.dir == "" ||
			strings.HasSuffix(d1.dirKind, "deploy directory") ||
			strings.HasSuffix(d2.dirKind, "deploy directory")
	}
	for i := 0; i < len(entries)-1; i++ {
		d1 := entries[i]
		for j := i + 1; j < len(entries); j++ {
			d2 := entries[j]
			if ignore(d1, d2) {
				continue
			}

			if utils.IsSubDir(d1.dir, d2.dir) || utils.IsSubDir(d2.dir, d1.dir) {
				// overlap is allowed in the case both sides are imported
				if d1.instance.IsImported() && d2.instance.IsImported() {
					continue
				}

				// overlap is allowed in the case one side is imported and the other is monitor,
				// we assume that the monitor is deployed with the first instance in that host,
				// it implies that the monitor is imported too.
				if (strings.HasPrefix(d1.dirKind, "monitor") && d2.instance.IsImported()) ||
					(d1.instance.IsImported() && strings.HasPrefix(d2.dirKind, "monitor")) {
					continue
				}

				// overlap is allowed in the case one side is data dir of a monitor instance,
				// as the *_exporter don't need data dir, the field is only kept for compatiability
				// with legacy tidb-ansible deployments.
				if (strings.HasPrefix(d1.dirKind, "monitor data directory")) ||
					(strings.HasPrefix(d2.dirKind, "monitor data directory")) {
					continue
				}

				properties := map[string]string{
					"ThisDirKind":   d1.dirKind,
					"ThisDir":       d1.dir,
					"ThisComponent": d1.instance.ComponentName(),
					"ThisHost":      d1.instance.GetHost(),
					"ThatDirKind":   d2.dirKind,
					"ThatDir":       d2.dir,
					"ThatComponent": d2.instance.ComponentName(),
					"ThatHost":      d2.instance.GetHost(),
				}
				zap.L().Info("Meet deploy directory overlap", zap.Any("info", properties))
				return errDeployDirOverlap.New("Deploy directory overlaps to another instance").WithProperty(tui.SuggestionFromTemplate(`
The directory you specified in the topology file is:
  Directory: {{ColorKeyword}}{{.ThisDirKind}} {{.ThisDir}}{{ColorReset}}
  Component: {{ColorKeyword}}{{.ThisComponent}} {{.ThisHost}}{{ColorReset}}

It overlaps to another instance:
  Other Directory: {{ColorKeyword}}{{.ThatDirKind}} {{.ThatDir}}{{ColorReset}}
  Other Component: {{ColorKeyword}}{{.ThatComponent}} {{.ThatHost}}{{ColorReset}}

Please modify the topology file and try again.
`, properties))
			}
		}
	}

	return nil
}

// CheckClusterPortConflict checks cluster port conflict
func CheckClusterPortConflict(clusterList map[string]Metadata, clusterName string, topo Topology) error {
	type Entry struct {
		clusterName   string
		componentName string
		port          int
		instance      Instance
	}

	currentEntries := []Entry{}
	existingEntries := []Entry{}

	for name, metadata := range clusterList {
		if name == clusterName {
			continue
		}

		uniqueHosts := set.NewStringSet()
		metadata.GetTopology().IterInstance(func(inst Instance) {
			mOpt := metadata.GetTopology().GetMonitoredOptions()
			if mOpt == nil {
				return
			}
			nodeExporterPort := mOpt.NodeExporterPort
			blackboxExporterPort := mOpt.BlackboxExporterPort
			for _, port := range inst.UsedPorts() {
				existingEntries = append(existingEntries, Entry{
					clusterName:   name,
					componentName: inst.ComponentName(),
					port:          port,
					instance:      inst,
				})
			}
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				existingEntries = append(existingEntries,
					Entry{
						clusterName:   name,
						componentName: RoleMonitor,
						port:          nodeExporterPort,
						instance:      inst,
					},
					Entry{
						clusterName:   name,
						componentName: RoleMonitor,
						port:          blackboxExporterPort,
						instance:      inst,
					})
			}
		})
	}

	uniqueHosts := set.NewStringSet()
	topo.IterInstance(func(inst Instance) {
		for _, port := range inst.UsedPorts() {
			currentEntries = append(currentEntries, Entry{
				componentName: inst.ComponentName(),
				port:          port,
				instance:      inst,
			})
		}

		mOpt := topo.GetMonitoredOptions()
		if mOpt == nil {
			return
		}
		if !uniqueHosts.Exist(inst.GetHost()) {
			uniqueHosts.Insert(inst.GetHost())
			currentEntries = append(currentEntries,
				Entry{
					componentName: RoleMonitor,
					port:          mOpt.NodeExporterPort,
					instance:      inst,
				},
				Entry{
					componentName: RoleMonitor,
					port:          mOpt.BlackboxExporterPort,
					instance:      inst,
				})
		}
	})

	for _, p1 := range currentEntries {
		for _, p2 := range existingEntries {
			if p1.instance.GetHost() != p2.instance.GetHost() {
				continue
			}

			if p1.port == p2.port {
				// build the conflict info
				properties := map[string]string{
					"ThisPort":       strconv.Itoa(p1.port),
					"ThisComponent":  p1.componentName,
					"ThisHost":       p1.instance.GetHost(),
					"ExistCluster":   p2.clusterName,
					"ExistPort":      strconv.Itoa(p2.port),
					"ExistComponent": p2.componentName,
					"ExistHost":      p2.instance.GetHost(),
				}
				// if one of the instances marks itself as ignore_exporter, do not report
				// the monitoring agent ports conflict and just skip
				if (p1.componentName == RoleMonitor || p2.componentName == RoleMonitor) &&
					(p1.instance.IgnoreMonitorAgent() || p2.instance.IgnoreMonitorAgent()) {
					zap.L().Debug("Ignored deploy port conflict", zap.Any("info", properties))
					continue
				}

				// build error message
				zap.L().Info("Meet deploy port conflict", zap.Any("info", properties))
				return errDeployPortConflict.New("Deploy port conflicts to an existing cluster").WithProperty(tui.SuggestionFromTemplate(`
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

// TiKVLabelError indicates that some TiKV servers don't have correct labels
type TiKVLabelError struct {
	TiKVInstances map[string][]error
}

// Error implements error
func (e *TiKVLabelError) Error() string {
	ids := []string{}
	for id := range e.TiKVInstances {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	str := ""
	for _, id := range ids {
		if len(e.TiKVInstances[id]) == 0 {
			continue
		}
		errs := []string{}
		for _, e := range e.TiKVInstances[id] {
			errs = append(errs, e.Error())
		}
		sort.Strings(errs)

		str += fmt.Sprintf("%s:\n", id)
		for _, e := range errs {
			str += fmt.Sprintf("\t%s\n", e)
		}
	}
	return str
}

// TiKVLabelProvider provides the store labels information
type TiKVLabelProvider interface {
	GetTiKVLabels() (map[string]map[string]string, []map[string]api.LabelInfo, error)
}

func getHostFromAddress(addr string) string {
	return strings.Split(addr, ":")[0]
}

// CheckTiKVLabels will check if tikv missing label or have wrong label
func CheckTiKVLabels(pdLocLabels []string, slp TiKVLabelProvider) error {
	lerr := &TiKVLabelError{
		TiKVInstances: make(map[string][]error),
	}
	lbs := set.NewStringSet(pdLocLabels...)
	hosts := make(map[string]int)

	storeLabels, _, err := slp.GetTiKVLabels()
	if err != nil {
		return err
	}
	for kv := range storeLabels {
		host := getHostFromAddress(kv)
		hosts[host]++
	}

	for kv, ls := range storeLabels {
		host := getHostFromAddress(kv)
		if len(ls) == 0 && hosts[host] > 1 {
			lerr.TiKVInstances[kv] = append(
				lerr.TiKVInstances[kv],
				errors.New("multiple TiKV instances are deployed at the same host but location label missing"),
			)
			continue
		}
		for lname := range ls {
			if !lbs.Exist(lname) {
				lerr.TiKVInstances[kv] = append(
					lerr.TiKVInstances[kv],
					fmt.Errorf("label name '%s' is not specified in pd config (replication.location-labels: %v)", lname, pdLocLabels),
				)
			}
		}
	}

	if len(lerr.TiKVInstances) == 0 {
		return nil
	}
	return lerr
}

// platformConflictsDetect checks for conflicts in topology for different OS / Arch
// set to the same host / IP
func (s *Specification) platformConflictsDetect() error {
	type (
		conflict struct {
			os   string
			arch string
			cfg  string
		}
	)

	platformStats := map[string]conflict{}
	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeOf(s).Elem()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			// skip nodes imported from TiDB-Ansible
			if compSpec.Addr().Interface().(InstanceSpec).IsImported() {
				continue
			}
			// check hostname
			host := compSpec.FieldByName("Host").String()
			cfg := strings.Split(topoType.Field(i).Tag.Get("yaml"), ",")[0] // without meta
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}

			// platform conflicts
			stat := conflict{
				cfg: cfg,
			}
			if j, found := findField(compSpec, "OS"); found {
				stat.os = compSpec.Field(j).String()
			}
			if j, found := findField(compSpec, "Arch"); found {
				stat.arch = compSpec.Field(j).String()
			}

			prev, exist := platformStats[host]
			if exist {
				if prev.os != stat.os || prev.arch != stat.arch {
					return &meta.ValidateErr{
						Type:   meta.TypeMismatch,
						Target: "platform",
						LHS:    fmt.Sprintf("%s:%s/%s", prev.cfg, prev.os, prev.arch),
						RHS:    fmt.Sprintf("%s:%s/%s", stat.cfg, stat.os, stat.arch),
						Value:  host,
					}
				}
			}
			platformStats[host] = stat
		}
	}
	return nil
}

func (s *Specification) portInvalidDetect() error {
	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeOf(s).Elem()

	checkPort := func(idx int, compSpec reflect.Value) error {
		compSpec = reflect.Indirect(compSpec)
		cfg := strings.Split(topoType.Field(idx).Tag.Get("yaml"), ",")[0]

		for i := 0; i < compSpec.NumField(); i++ {
			if strings.HasSuffix(compSpec.Type().Field(i).Name, "Port") {
				port := int(compSpec.Field(i).Int())
				// for NgPort, 0 means default and -1 means disable
				if compSpec.Type().Field(i).Name == "NgPort" && (port == -1 || port == 0) {
					continue
				}
				if port < 1 || port > 65535 {
					portField := strings.Split(compSpec.Type().Field(i).Tag.Get("yaml"), ",")[0]
					return errors.Errorf("`%s` of %s=%d is invalid, port should be in the range [1, 65535]", cfg, portField, port)
				}
			}
		}
		return nil
	}

	for i := 0; i < topoSpec.NumField(); i++ {
		compSpecs := topoSpec.Field(i)

		// check on struct
		if compSpecs.Kind() == reflect.Struct {
			if err := checkPort(i, compSpecs); err != nil {
				return err
			}
			continue
		}

		// check on slice
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			if err := checkPort(i, compSpec); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Specification) portConflictsDetect() error {
	type (
		usedPort struct {
			host string
			port int
		}
		conflict struct {
			tp  string
			cfg string
		}
	)

	portTypes := []string{
		"Port",
		"StatusPort",
		"PeerPort",
		"ClientPort",
		"WebPort",
		"TCPPort",
		"HTTPPort",
		"FlashServicePort",
		"FlashProxyPort",
		"FlashProxyStatusPort",
		"ClusterPort",
		"NgPort",
	}

	portStats := map[usedPort]conflict{}
	uniqueHosts := set.NewStringSet()
	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeOf(s).Elem()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))

			// check hostname
			host := compSpec.FieldByName("Host").String()
			cfg := strings.Split(topoType.Field(i).Tag.Get("yaml"), ",")[0] // without meta
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}
			uniqueHosts.Insert(host)

			// Ports conflicts
			for _, portType := range portTypes {
				if j, found := findField(compSpec, portType); found {
					item := usedPort{
						host: host,
						port: int(compSpec.Field(j).Int()),
					}
					tp := compSpec.Type().Field(j).Tag.Get("yaml")
					prev, exist := portStats[item]
					if exist {
						return &meta.ValidateErr{
							Type:   meta.TypeConflict,
							Target: "port",
							LHS:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
							RHS:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
							Value:  item.port,
						}
					}
					portStats[item] = conflict{
						tp:  tp,
						cfg: cfg,
					}
				}
			}
		}
	}

	// Port conflicts in monitored components
	monitoredPortTypes := []string{
		"NodeExporterPort",
		"BlackboxExporterPort",
	}
	monitoredOpt := topoSpec.FieldByName(monitorOptionTypeName)
	for host := range uniqueHosts {
		cfg := "monitored"
		for _, portType := range monitoredPortTypes {
			f := monitoredOpt.FieldByName(portType)
			item := usedPort{
				host: host,
				port: int(f.Int()),
			}
			ft, found := monitoredOpt.Type().FieldByName(portType)
			if !found {
				return errors.Errorf("incompatible change `%s.%s`", monitorOptionTypeName, portType)
			}
			// `yaml:"node_exporter_port,omitempty"`
			tp := strings.Split(ft.Tag.Get("yaml"), ",")[0]
			prev, exist := portStats[item]
			if exist {
				return &meta.ValidateErr{
					Type:   meta.TypeConflict,
					Target: "port",
					LHS:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
					RHS:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
					Value:  item.port,
				}
			}
			portStats[item] = conflict{
				tp:  tp,
				cfg: cfg,
			}
		}
	}

	return nil
}

func (s *Specification) dirConflictsDetect() error {
	type (
		usedDir struct {
			host string
			dir  string
		}
		conflict struct {
			tp       string
			cfg      string
			imported bool
		}
	)

	dirTypes := []string{
		"DataDir",
		"DeployDir",
	}

	// usedInfo => type
	var dirStats = map[usedDir]conflict{}

	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeOf(s).Elem()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			// check hostname
			host := compSpec.FieldByName("Host").String()
			cfg := strings.Split(topoType.Field(i).Tag.Get("yaml"), ",")[0] // without meta
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}

			// Directory conflicts
			for _, dirType := range dirTypes {
				j, found := findField(compSpec, dirType)
				if !found {
					continue
				}

				// `yaml:"data_dir,omitempty"`
				tp := strings.Split(compSpec.Type().Field(j).Tag.Get("yaml"), ",")[0]
				for _, dir := range strings.Split(compSpec.Field(j).String(), ",") {
					dir = strings.TrimSpace(dir)
					item := usedDir{
						host: host,
						dir:  dir,
					}
					// data_dir is relative to deploy_dir by default, so they can be with
					// same (sub) paths as long as the deploy_dirs are different
					if item.dir != "" && !strings.HasPrefix(item.dir, "/") {
						continue
					}
					prev, exist := dirStats[item]
					// not checking between imported nodes
					if exist &&
						!(compSpec.Addr().Interface().(InstanceSpec).IsImported() && prev.imported) {
						return &meta.ValidateErr{
							Type:   meta.TypeConflict,
							Target: "directory",
							LHS:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
							RHS:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
							Value:  item.dir,
						}
					}
					// not reporting error for nodes imported from TiDB-Ansible, but keep
					// their dirs in the map to check if other nodes are using them
					dirStats[item] = conflict{
						tp:       tp,
						cfg:      cfg,
						imported: compSpec.Addr().Interface().(InstanceSpec).IsImported(),
					}
				}
			}
		}
	}

	return nil
}

// CountDir counts for dir paths used by any instance in the cluster with the same
// prefix, useful to find potential path conflicts
func (s *Specification) CountDir(targetHost, dirPrefix string) int {
	dirTypes := []string{
		"DeployDir",
		"DataDir",
		"LogDir",
	}

	// path -> count
	dirStats := make(map[string]int)
	count := 0
	topoSpec := reflect.ValueOf(s).Elem()
	dirPrefix = Abs(s.GlobalOptions.User, dirPrefix)

	addHostDir := func(host, deployDir, dir string) {
		if !strings.HasPrefix(dir, "/") {
			dir = filepath.Join(deployDir, dir)
		}
		dir = Abs(s.GlobalOptions.User, dir)
		dirStats[host+dir]++
	}

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			deployDir := compSpec.FieldByName("DeployDir").String()
			host := compSpec.FieldByName("Host").String()

			for _, dirType := range dirTypes {
				j, found := findField(compSpec, dirType)
				if !found {
					continue
				}

				dir := compSpec.Field(j).String()

				switch dirType { // the same as in instance.go for (*instance)
				case "DeployDir":
					addHostDir(host, deployDir, "")
				case "DataDir":
					// the default data_dir is relative to deploy_dir
					if dir == "" {
						addHostDir(host, deployDir, dir)
						continue
					}
					for _, dataDir := range strings.Split(dir, ",") {
						dataDir = strings.TrimSpace(dataDir)
						if dataDir != "" {
							addHostDir(host, deployDir, dataDir)
						}
					}
				case "LogDir":
					field := compSpec.FieldByName("LogDir")
					if field.IsValid() {
						dir = field.Interface().(string)
					}

					if dir == "" {
						dir = "log"
					}
					addHostDir(host, deployDir, strings.TrimSpace(dir))
				}
			}
		}
	}

	for k, v := range dirStats {
		if k == targetHost+dirPrefix || strings.HasPrefix(k, targetHost+dirPrefix+"/") {
			count += v
		}
	}

	return count
}

func (s *Specification) validateTiSparkSpec() error {
	// There must be a Spark master
	if len(s.TiSparkMasters) == 0 {
		if len(s.TiSparkWorkers) == 0 {
			return nil
		}
		return ErrNoTiSparkMaster
	}

	// We only support 1 Spark master at present
	if len(s.TiSparkMasters) > 1 {
		return ErrMultipleTiSparkMaster
	}

	// Multiple workers on the same host is not supported by Spark
	if len(s.TiSparkWorkers) > 1 {
		cnt := make(map[string]int)
		for _, w := range s.TiSparkWorkers {
			if cnt[w.Host] > 0 {
				return errors.Annotatef(ErrMultipleTisparkWorker, "the host %s is duplicated", w.Host)
			}
			cnt[w.Host]++
		}
	}

	return nil
}

func (s *Specification) validateTLSEnabled() error {
	if !s.GlobalOptions.TLSEnabled {
		return nil
	}

	// check for component with no tls support
	compList := make([]Component, 0)
	s.IterComponent(func(c Component) {
		if len(c.Instances()) > 0 {
			compList = append(compList, c)
		}
	})

	for _, c := range compList {
		switch c.Name() {
		case ComponentPD,
			ComponentTiDB,
			ComponentTiKV,
			ComponentTiFlash,
			ComponentPump,
			ComponentDrainer,
			ComponentCDC,
			ComponentPrometheus,
			ComponentAlertmanager,
			ComponentGrafana:
		default:
			return errors.Errorf("component %s is not supported in TLS enabled cluster", c.Name())
		}
	}
	return nil
}

func (s *Specification) validateUserGroup() error {
	gOpts := s.GlobalOptions
	if user := gOpts.User; !reUser.MatchString(user) {
		return errors.Annotatef(ErrUserOrGroupInvalid, "`global` of user='%s' is invalid", user)
	}
	// if group is nil, then we'll set it to the same as user
	if group := gOpts.Group; group != "" && !reGroup.MatchString(group) {
		return errors.Annotatef(ErrUserOrGroupInvalid, "`global` of group='%s' is invalid", group)
	}
	return nil
}

func (s *Specification) validatePDNames() error {
	// check pdserver name
	pdNames := set.NewStringSet()
	for _, pd := range s.PDServers {
		if pd.Name == "" {
			continue
		}

		if pdNames.Exist(pd.Name) {
			return errors.Errorf("component pd_servers.name is not supported duplicated, the name %s is duplicated", pd.Name)
		}
		pdNames.Insert(pd.Name)
	}
	return nil
}

func (s *Specification) validateTiFlashConfigs() error {
	c := FindComponent(s, ComponentTiFlash)
	for _, ins := range c.Instances() {
		if err := ins.(*TiFlashInstance).CheckIncorrectConfigs(); err != nil {
			return err
		}
	}
	return nil
}

// validateMonitorAgent checks for conflicts in topology for different ignore_exporter
// settings for multiple instances on the same host / IP
func (s *Specification) validateMonitorAgent() error {
	type (
		conflict struct {
			ignore bool
			cfg    string
		}
	)
	agentStats := map[string]conflict{}
	topoSpec := reflect.ValueOf(s).Elem()
	topoType := reflect.TypeOf(s).Elem()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			// skip nodes imported from TiDB-Ansible
			if compSpec.Addr().Interface().(InstanceSpec).IsImported() {
				continue
			}

			// check hostname
			host := compSpec.FieldByName("Host").String()
			cfg := strings.Split(topoType.Field(i).Tag.Get("yaml"), ",")[0] // without meta
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}

			// agent conflicts
			stat := conflict{}
			if j, found := findField(compSpec, "IgnoreExporter"); found {
				stat.ignore = compSpec.Field(j).Bool()
				stat.cfg = cfg
			}

			prev, exist := agentStats[host]
			if exist {
				if prev.ignore != stat.ignore {
					return &meta.ValidateErr{
						Type:   meta.TypeMismatch,
						Target: "ignore_exporter",
						LHS:    fmt.Sprintf("%s:%v", prev.cfg, prev.ignore),
						RHS:    fmt.Sprintf("%s:%v", stat.cfg, stat.ignore),
						Value:  host,
					}
				}
			}
			agentStats[host] = stat
		}
	}
	return nil
}

// Validate validates the topology specification and produce error if the
// specification invalid (e.g: port conflicts or directory conflicts)
func (s *Specification) Validate() error {
	validators := []func() error{
		s.validateTLSEnabled,
		s.platformConflictsDetect,
		s.portInvalidDetect,
		s.portConflictsDetect,
		s.dirConflictsDetect,
		s.validateUserGroup,
		s.validatePDNames,
		s.validateTiSparkSpec,
		s.validateTiFlashConfigs,
		s.validateMonitorAgent,
	}

	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}

	return RelativePathDetect(s, isSkipField)
}

// RelativePathDetect detect if some specific path is relative path and report error
func RelativePathDetect(topo interface{}, isSkipField func(reflect.Value) bool) error {
	pathTypes := []string{
		"ConfigFilePath",
		"RuleDir",
		"DashboardDir",
	}

	topoSpec := reflect.ValueOf(topo).Elem()

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))

			// Relateve path detect
			for _, field := range pathTypes {
				if j, found := findField(compSpec, field); found {
					// `yaml:"xxxx,omitempty"`
					fieldName := strings.Split(compSpec.Type().Field(j).Tag.Get("yaml"), ",")[0]
					localPath := compSpec.Field(j).String()
					if localPath != "" && !strings.HasPrefix(localPath, "/") {
						return fmt.Errorf("relative path is not allowed for field %s: %s", fieldName, localPath)
					}
				}
			}
		}
	}

	return nil
}
