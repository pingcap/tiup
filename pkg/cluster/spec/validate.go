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
	"sort"
	"strconv"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/errutil"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"go.uber.org/zap"
)

// pre defined error types
var (
	errNSDeploy              = errNS.NewSubNamespace("deploy")
	errDeployDirConflict     = errNSDeploy.NewType("dir_conflict", errutil.ErrTraitPreCheck)
	errDeployPortConflict    = errNSDeploy.NewType("port_conflict", errutil.ErrTraitPreCheck)
	ErrNoTiSparkMaster       = errors.New("there must be a Spark master node if you want to use the TiSpark component")
	ErrMultipleTiSparkMaster = errors.New("a TiSpark enabled cluster with more than 1 Spark master node is not supported")
	ErrMultipleTisparkWorker = errors.New("multiple TiSpark workers on the same host is not supported by Spark")
)

func fixDir(topo Topology) func(string) string {
	return func(dir string) string {
		if dir != "" {
			return Abs(topo.BaseTopo().GlobalOptions.User, dir)
		}
		return dir
	}
}

// CheckClusterDirConflict checks cluster dir conflict
func CheckClusterDirConflict(clusterList map[string]Metadata, clusterName string, topo Topology) error {
	type DirAccessor struct {
		dirKind  string
		accessor func(Instance, Topology) string
	}

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
			return topo.BaseTopo().MonitoredOptions.DeployDir
		}},
		{dirKind: "monitor data directory", accessor: func(instance Instance, topo Topology) string {
			m := topo.BaseTopo().MonitoredOptions
			if m == nil {
				return ""
			}
			return topo.BaseTopo().MonitoredOptions.DataDir
		}},
		{dirKind: "monitor log directory", accessor: func(instance Instance, topo Topology) string {
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
		instance    Instance
	}

	currentEntries := []Entry{}
	existingEntries := []Entry{}

	for name, metadata := range clusterList {
		if name == clusterName {
			continue
		}

		topo := metadata.GetTopology()

		f := fixDir(topo)
		topo.IterInstance(func(inst Instance) {
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
		IterHost(topo, func(inst Instance) {
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
	topo.IterInstance(func(inst Instance) {
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

	IterHost(topo, func(inst Instance) {
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
func CheckClusterPortConflict(clusterList map[string]Metadata, clusterName string, topo Topology) error {
	type Entry struct {
		clusterName string
		instance    Instance
		port        int
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
					clusterName: name,
					instance:    inst,
					port:        port,
				})
			}
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				existingEntries = append(existingEntries,
					Entry{
						clusterName: name,
						instance:    inst,
						port:        nodeExporterPort,
					},
					Entry{
						clusterName: name,
						instance:    inst,
						port:        blackboxExporterPort,
					})
			}
		})
	}

	uniqueHosts := set.NewStringSet()
	topo.IterInstance(func(inst Instance) {
		for _, port := range inst.UsedPorts() {
			currentEntries = append(currentEntries, Entry{
				instance: inst,
				port:     port,
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
					instance: inst,
					port:     mOpt.NodeExporterPort,
				},
				Entry{
					instance: inst,
					port:     mOpt.BlackboxExporterPort,
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

// StoreLabelProvider provide store labels information
type StoreLabelProvider interface {
	StoreList() ([]string, error)
	GetStoreLabels(address string) (map[string]string, error)
}

func getHostFromAddress(addr string) string {
	return strings.Split(addr, ":")[0]
}

// CheckTiKVLocationLabels will check if tikv missing label or have wrong label
func CheckTiKVLocationLabels(pdLocLabels []string, slp StoreLabelProvider) error {
	lerr := &TiKVLabelError{
		TiKVInstances: make(map[string][]error),
	}
	lbs := set.NewStringSet(pdLocLabels...)
	hosts := make(map[string]int)

	kvs, err := slp.StoreList()
	if err != nil {
		return err
	}
	for _, kv := range kvs {
		host := getHostFromAddress(kv)
		hosts[host] = hosts[host] + 1
	}
	for _, kv := range kvs {
		host := getHostFromAddress(kv)
		ls, err := slp.GetStoreLabels(kv)
		if err != nil {
			return err
		}
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
			compSpec := compSpecs.Index(index)
			// skip nodes imported from TiDB-Ansible
			if compSpec.Interface().(InstanceSpec).IsImported() {
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
		"ClusterPort",
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
			compSpec := compSpecs.Index(index)
			// skip nodes imported from TiDB-Ansible
			if compSpec.Interface().(InstanceSpec).IsImported() {
				continue
			}
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
			compSpec := compSpecs.Index(index)
			// check hostname
			host := compSpec.FieldByName("Host").String()
			cfg := strings.Split(topoType.Field(i).Tag.Get("yaml"), ",")[0] // without meta
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}

			// Directory conflicts
			for _, dirType := range dirTypes {
				if j, found := findField(compSpec, dirType); found {
					item := usedDir{
						host: host,
						dir:  compSpec.Field(j).String(),
					}
					// data_dir is relative to deploy_dir by default, so they can be with
					// same (sub) paths as long as the deploy_dirs are different
					if item.dir != "" && !strings.HasPrefix(item.dir, "/") {
						continue
					}
					// `yaml:"data_dir,omitempty"`
					tp := strings.Split(compSpec.Type().Field(j).Tag.Get("yaml"), ",")[0]
					prev, exist := dirStats[item]
					// not checking between imported nodes
					if exist &&
						!(compSpec.Interface().(InstanceSpec).IsImported() && prev.imported) {
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
						imported: compSpec.Interface().(InstanceSpec).IsImported(),
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
		"DataDir",
		"DeployDir",
		"LogDir",
	}

	// path -> count
	dirStats := make(map[string]int)
	count := 0
	topoSpec := reflect.ValueOf(s).Elem()
	dirPrefix = Abs(s.GlobalOptions.User, dirPrefix)

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := compSpecs.Index(index)
			// Directory conflicts
			for _, dirType := range dirTypes {
				if j, found := findField(compSpec, dirType); found {
					dir := compSpec.Field(j).String()
					host := compSpec.FieldByName("Host").String()

					switch dirType { // the same as in instance.go for (*instance)
					case "DataDir":
						deployDir := compSpec.FieldByName("DeployDir").String()
						// the default data_dir is relative to deploy_dir
						if dir != "" && !strings.HasPrefix(dir, "/") {
							dir = filepath.Join(deployDir, dir)
						}
					case "LogDir":
						deployDir := compSpec.FieldByName("DeployDir").String()
						field := compSpec.FieldByName("LogDir")
						if field.IsValid() {
							dir = field.Interface().(string)
						}

						if dir == "" {
							dir = "log"
						}
						if !strings.HasPrefix(dir, "/") {
							dir = filepath.Join(deployDir, dir)
						}
					}
					dir = Abs(s.GlobalOptions.User, dir)
					dirStats[host+dir]++
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
			ComponentPump,
			ComponentDrainer,
			ComponentCDC,
			ComponentPrometheus,
			ComponentAlertManager,
			ComponentGrafana:
		default:
			return errors.Errorf("component %s is not supported in TLS enabled cluster", c.Name())
		}
	}
	return nil
}

// Validate validates the topology specification and produce error if the
// specification invalid (e.g: port conflicts or directory conflicts)
func (s *Specification) Validate() error {
	if err := s.validateTLSEnabled(); err != nil {
		return err
	}

	if err := s.platformConflictsDetect(); err != nil {
		return err
	}

	if err := s.portConflictsDetect(); err != nil {
		return err
	}

	if err := s.dirConflictsDetect(); err != nil {
		return err
	}

	if err := s.validateTiSparkSpec(); err != nil {
		return err
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
			compSpec := compSpecs.Index(index)

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
