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
	"crypto/tls"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
)

const (
	statusQueryTimeout = 2 * time.Second
)

var (
	globalOptionTypeName  = reflect.TypeOf(GlobalOptions{}).Name()
	monitorOptionTypeName = reflect.TypeOf(MonitoredOptions{}).Name()
	serverConfigsTypeName = reflect.TypeOf(DMServerConfigs{}).Name()
)

func setDefaultDir(parent, role, port string, field reflect.Value) {
	if field.String() != "" {
		return
	}
	if defaults.CanUpdate(field.Interface()) {
		dir := fmt.Sprintf("%s-%s", role, port)
		field.Set(reflect.ValueOf(filepath.Join(parent, dir)))
	}
}

func findField(v reflect.Value, fieldName string) (int, bool) {
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == fieldName {
			return i, true
		}
	}
	return -1, false
}

// Skip global/monitored/job options
func isSkipField(field reflect.Value) bool {
	tp := field.Type().Name()
	return tp == globalOptionTypeName || tp == monitorOptionTypeName || tp == serverConfigsTypeName
}

type (
	// GlobalOptions of spec.
	GlobalOptions = spec.GlobalOptions
	// MonitoredOptions is the spec of Monitored
	MonitoredOptions = spec.MonitoredOptions
	// PrometheusSpec is the spec of Prometheus
	PrometheusSpec = spec.PrometheusSpec
	// GrafanaSpec is the spec of Grafana
	GrafanaSpec = spec.GrafanaSpec
	// AlertmanagerSpec is the spec of Alertmanager
	AlertmanagerSpec = spec.AlertmanagerSpec
	// ResourceControl is the spec of ResourceControl
	ResourceControl = meta.ResourceControl
)

type (
	// DMServerConfigs represents the server runtime configuration
	DMServerConfigs struct {
		Master map[string]interface{} `yaml:"master"`
		Worker map[string]interface{} `yaml:"worker"`
	}

	// Specification represents the specification of topology.yaml
	Specification struct {
		GlobalOptions    GlobalOptions            `yaml:"global,omitempty" validate:"global:editable"`
		MonitoredOptions MonitoredOptions         `yaml:"monitored,omitempty" validate:"monitored:editable"`
		ServerConfigs    DMServerConfigs          `yaml:"server_configs,omitempty" validate:"server_configs:ignore"`
		Masters          []*MasterSpec            `yaml:"master_servers"`
		Workers          []*WorkerSpec            `yaml:"worker_servers"`
		Monitors         []*spec.PrometheusSpec   `yaml:"monitoring_servers"`
		Grafanas         []*spec.GrafanaSpec      `yaml:"grafana_servers,omitempty"`
		Alertmanagers    []*spec.AlertmanagerSpec `yaml:"alertmanager_servers,omitempty"`
	}
)

// AllDMComponentNames contains the names of all dm components.
// should include all components in ComponentsByStartOrder
func AllDMComponentNames() (roles []string) {
	tp := &Specification{}
	tp.IterComponent(func(c Component) {
		roles = append(roles, c.Name())
	})

	return
}

// MasterSpec represents the Master topology specification in topology.yaml
type MasterSpec struct {
	Host     string `yaml:"host"`
	SSHPort  int    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported bool   `yaml:"imported,omitempty"`
	Patched  bool   `yaml:"patched,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name            string                 `yaml:"name,omitempty"`
	Port            int                    `yaml:"port,omitempty" default:"8261"`
	PeerPort        int                    `yaml:"peer_port,omitempty" default:"8291"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
	V1SourcePath    string                 `yaml:"v1_source_path,omitempty"`
}

// Status queries current status of the instance
func (s *MasterSpec) Status(tlsCfg *tls.Config, _ ...string) string {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	dc := api.NewDMMasterClient([]string{addr}, statusQueryTimeout, tlsCfg)
	isFound, isActive, isLeader, err := dc.GetMaster(s.Name)
	if err != nil {
		return "Down"
	}
	if !isFound {
		return "N/A"
	}
	if !isActive {
		return "Unhealthy"
	}
	res := "Healthy"
	if isLeader {
		res += "|L"
	}
	return res
}

// Role returns the component role of the instance
func (s *MasterSpec) Role() string {
	return ComponentDMMaster
}

// SSH returns the host and SSH port of the instance
func (s *MasterSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *MasterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *MasterSpec) IsImported() bool {
	return s.Imported
}

// WorkerSpec represents the Master topology specification in topology.yaml
type WorkerSpec struct {
	Host     string `yaml:"host"`
	SSHPort  int    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported bool   `yaml:"imported,omitempty"`
	Patched  bool   `yaml:"patched,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name            string                 `yaml:"name,omitempty"`
	Port            int                    `yaml:"port,omitempty" default:"8262"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s *WorkerSpec) Status(tlsCfg *tls.Config, masterList ...string) string {
	if len(masterList) < 1 {
		return "N/A"
	}
	dc := api.NewDMMasterClient(masterList, statusQueryTimeout, tlsCfg)
	stage, err := dc.GetWorker(s.Name)
	if err != nil {
		return "Down"
	}
	if stage == "" {
		return "N/A"
	}
	return stage
}

// Role returns the component role of the instance
func (s *WorkerSpec) Role() string {
	return ComponentDMWorker
}

// SSH returns the host and SSH port of the instance
func (s *WorkerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *WorkerSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *WorkerSpec) IsImported() bool {
	return s.Imported
}

// UnmarshalYAML sets default values when unmarshaling the topology file
func (s *Specification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type topology Specification
	if err := unmarshal((*topology)(s)); err != nil {
		return err
	}

	if err := defaults.Set(s); err != nil {
		return errors.Trace(err)
	}

	// Set monitored options
	if s.MonitoredOptions.DeployDir == "" {
		s.MonitoredOptions.DeployDir = filepath.Join(s.GlobalOptions.DeployDir,
			fmt.Sprintf("%s-%d", spec.RoleMonitor, s.MonitoredOptions.NodeExporterPort))
	}
	if s.MonitoredOptions.DataDir == "" {
		s.MonitoredOptions.DataDir = filepath.Join(s.GlobalOptions.DataDir,
			fmt.Sprintf("%s-%d", spec.RoleMonitor, s.MonitoredOptions.NodeExporterPort))
	}
	if s.MonitoredOptions.LogDir == "" {
		s.MonitoredOptions.LogDir = "log"
	}
	if !strings.HasPrefix(s.MonitoredOptions.LogDir, "/") &&
		!strings.HasPrefix(s.MonitoredOptions.LogDir, s.MonitoredOptions.DeployDir) {
		s.MonitoredOptions.LogDir = filepath.Join(s.MonitoredOptions.DeployDir, s.MonitoredOptions.LogDir)
	}

	if err := fillDMCustomDefaults(&s.GlobalOptions, s); err != nil {
		return err
	}

	return s.Validate()
}

// platformConflictsDetect checks for conflicts in topology for different OS / Arch
// for set to the same host / IP
func (topo *Specification) platformConflictsDetect() error {
	type (
		conflict struct {
			os   string
			arch string
			cfg  string
		}
	)

	platformStats := map[string]conflict{}
	topoSpec := reflect.ValueOf(topo).Elem()
	topoType := reflect.TypeOf(topo).Elem()

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
			cfg := topoType.Field(i).Tag.Get("yaml")
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

func (topo *Specification) portConflictsDetect() error {
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
	topoSpec := reflect.ValueOf(topo).Elem()
	topoType := reflect.TypeOf(topo).Elem()

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
			cfg := topoType.Field(i).Tag.Get("yaml")
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

	return nil
}

func (topo *Specification) dirConflictsDetect() error {
	type (
		usedDir struct {
			host string
			dir  string
		}
		conflict struct {
			tp  string
			cfg string
		}
	)

	dirTypes := []string{
		"DataDir",
		"DeployDir",
	}

	// usedInfo => type
	var (
		dirStats    = map[usedDir]conflict{}
		uniqueHosts = set.NewStringSet()
	)

	topoSpec := reflect.ValueOf(topo).Elem()
	topoType := reflect.TypeOf(topo).Elem()

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
			cfg := topoType.Field(i).Tag.Get("yaml")
			if host == "" {
				return errors.Errorf("`%s` contains empty host field", cfg)
			}
			uniqueHosts.Insert(host)

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
					if exist {
						return &meta.ValidateErr{
							Type:   meta.TypeConflict,
							Target: "directory",
							LHS:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
							RHS:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
							Value:  item.dir,
						}
					}
					dirStats[item] = conflict{
						tp:  tp,
						cfg: cfg,
					}
				}
			}
		}
	}

	return nil
}

// CountDir counts for dir paths used by any instance in the cluster with the same
// prefix, useful to find potential path conflicts
func (topo *Specification) CountDir(targetHost, dirPrefix string) int {
	dirTypes := []string{
		"DataDir",
		"DeployDir",
		"LogDir",
	}

	// host-path -> count
	dirStats := make(map[string]int)
	count := 0
	topoSpec := reflect.ValueOf(topo).Elem()
	dirPrefix = spec.Abs(topo.GlobalOptions.User, dirPrefix)

	for i := 0; i < topoSpec.NumField(); i++ {
		if isSkipField(topoSpec.Field(i)) {
			continue
		}

		compSpecs := topoSpec.Field(i)
		for index := 0; index < compSpecs.Len(); index++ {
			compSpec := reflect.Indirect(compSpecs.Index(index))
			// Directory conflicts
			for _, dirType := range dirTypes {
				if j, found := findField(compSpec, dirType); found {
					dir := compSpec.Field(j).String()
					host := compSpec.FieldByName("Host").String()

					switch dirType { // the same as in logic.go for (*instance)
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
					dir = spec.Abs(topo.GlobalOptions.User, dir)
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

// TLSConfig generates a tls.Config for the specification as needed
func (topo *Specification) TLSConfig(dir string) (*tls.Config, error) {
	if !topo.GlobalOptions.TLSEnabled {
		return nil, nil
	}
	return spec.LoadClientCert(dir)
}

// Validate validates the topology specification and produce error if the
// specification invalid (e.g: port conflicts or directory conflicts)
func (topo *Specification) Validate() error {
	if err := topo.platformConflictsDetect(); err != nil {
		return err
	}

	if err := topo.portConflictsDetect(); err != nil {
		return err
	}

	if err := topo.dirConflictsDetect(); err != nil {
		return err
	}

	return spec.RelativePathDetect(topo, isSkipField)
}

// Type implements Topology interface.
func (topo *Specification) Type() string {
	return spec.TopoTypeDM
}

// BaseTopo implements Topology interface.
func (topo *Specification) BaseTopo() *spec.BaseTopo {
	return &spec.BaseTopo{
		GlobalOptions:    &topo.GlobalOptions,
		MonitoredOptions: topo.GetMonitoredOptions(),
		MasterList:       topo.GetMasterList(),
		Monitors:         topo.Monitors,
		Grafanas:         topo.Grafanas,
		Alertmanagers:    topo.Alertmanagers,
	}
}

// NewPart implements ScaleOutTopology interface.
func (topo *Specification) NewPart() spec.Topology {
	return &Specification{
		GlobalOptions: topo.GlobalOptions,
		ServerConfigs: topo.ServerConfigs,
	}
}

// MergeTopo implements ScaleOutTopology interface.
func (topo *Specification) MergeTopo(rhs spec.Topology) spec.Topology {
	other, ok := rhs.(*Specification)
	if !ok {
		panic("topo should be DM Topology")
	}

	return topo.Merge(other)
}

// GetMasterList returns a list of Master API hosts of the current cluster
func (topo *Specification) GetMasterList() []string {
	var masterList []string

	for _, master := range topo.Masters {
		masterList = append(masterList, fmt.Sprintf("%s:%d", master.Host, master.Port))
	}

	return masterList
}

// Merge returns a new Topology which sum old ones
func (topo *Specification) Merge(that spec.Topology) spec.Topology {
	spec := that.(*Specification)
	return &Specification{
		GlobalOptions:    topo.GlobalOptions,
		MonitoredOptions: topo.MonitoredOptions,
		ServerConfigs:    topo.ServerConfigs,
		Masters:          append(topo.Masters, spec.Masters...),
		Workers:          append(topo.Workers, spec.Workers...),
		Monitors:         append(topo.Monitors, spec.Monitors...),
		Grafanas:         append(topo.Grafanas, spec.Grafanas...),
		Alertmanagers:    append(topo.Alertmanagers, spec.Alertmanagers...),
	}
}

// fillDefaults tries to fill custom fields to their default values
func fillDMCustomDefaults(globalOptions *GlobalOptions, data interface{}) error {
	v := reflect.ValueOf(data).Elem()
	t := v.Type()

	var err error
	for i := 0; i < t.NumField(); i++ {
		if err = setDMCustomDefaults(globalOptions, v.Field(i)); err != nil {
			return err
		}
	}

	return nil
}

func setDMCustomDefaults(globalOptions *GlobalOptions, field reflect.Value) error {
	if !field.CanSet() || isSkipField(field) {
		return nil
	}

	switch field.Kind() {
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			if err := setDMCustomDefaults(globalOptions, field.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		ref := reflect.New(field.Type())
		ref.Elem().Set(field)
		if err := fillDMCustomDefaults(globalOptions, ref.Interface()); err != nil {
			return err
		}
		field.Set(ref.Elem())
	case reflect.Ptr:
		if err := setDMCustomDefaults(globalOptions, field.Elem()); err != nil {
			return err
		}
	}

	if field.Kind() != reflect.Struct {
		return nil
	}

	for j := 0; j < field.NumField(); j++ {
		switch field.Type().Field(j).Name {
		case "SSHPort":
			if field.Field(j).Int() != 0 {
				continue
			}
			field.Field(j).Set(reflect.ValueOf(globalOptions.SSHPort))
		case "Name":
			if field.Field(j).String() != "" {
				continue
			}
			host := field.FieldByName("Host").String()
			port := field.FieldByName("Port").Int()
			field.Field(j).Set(reflect.ValueOf(fmt.Sprintf("dm-%s-%d", host, port)))
		case "DataDir":
			dataDir := field.Field(j).String()
			if dataDir != "" { // already have a value, skip filling default values
				continue
			}
			// If the data dir in global options is an obsolute path, it appends to
			// the global and has a comp-port sub directory
			if strings.HasPrefix(globalOptions.DataDir, "/") {
				field.Field(j).Set(reflect.ValueOf(filepath.Join(
					globalOptions.DataDir,
					fmt.Sprintf("%s-%s", field.Addr().Interface().(InstanceSpec).Role(), getPort(field)),
				)))
				continue
			}

			// If the data dir in global options is empty or a relative path, keep it be relative
			// Our run_*.sh start scripts are run inside deploy_path, so the final location
			// will be deploy_path/global.data_dir
			// (the default value of global.data_dir is "data")
			if globalOptions.DataDir == "" {
				field.Field(j).Set(reflect.ValueOf("data"))
			} else {
				field.Field(j).Set(reflect.ValueOf(globalOptions.DataDir))
			}
		case "DeployDir":
			setDefaultDir(globalOptions.DeployDir, field.Addr().Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
		case "LogDir":
			if field.Field(j).String() == "" && defaults.CanUpdate(field.Field(j).Interface()) {
				field.Field(j).Set(reflect.ValueOf(globalOptions.LogDir))
			}
		case "Arch":
			// default values of globalOptions are set before fillCustomDefaults in Unmarshal
			// so the globalOptions.Arch already has its default value set, no need to check again
			if field.Field(j).String() == "" {
				field.Field(j).Set(reflect.ValueOf(globalOptions.Arch))
			}

			switch strings.ToLower(field.Field(j).String()) {
			// replace "x86_64" with amd64, they are the same in our repo
			case "x86_64":
				field.Field(j).Set(reflect.ValueOf("amd64"))
			// replace "aarch64" with arm64
			case "aarch64":
				field.Field(j).Set(reflect.ValueOf("arm64"))
			}

			// convert to lower case
			if field.Field(j).String() != "" {
				field.Field(j).Set(reflect.ValueOf(strings.ToLower(field.Field(j).String())))
			}
		case "OS":
			// default value of globalOptions.OS is already set, same as "Arch"
			if field.Field(j).String() == "" {
				field.Field(j).Set(reflect.ValueOf(globalOptions.OS))
			}
			// convert to lower case
			if field.Field(j).String() != "" {
				field.Field(j).Set(reflect.ValueOf(strings.ToLower(field.Field(j).String())))
			}
		}
	}

	return nil
}

func getPort(v reflect.Value) string {
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "Port", "ClientPort", "WebPort", "TCPPort", "NodeExporterPort":
			return fmt.Sprintf("%d", v.Field(i).Int())
		}
	}
	return ""
}
