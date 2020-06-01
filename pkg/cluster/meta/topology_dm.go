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

package meta

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/creasty/defaults"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/set"
)

type (
	// DMServerConfigs represents the server runtime configuration
	DMServerConfigs struct {
		Master map[string]interface{} `yaml:"dm-master"`
		Worker map[string]interface{} `yaml:"dm-worker"`
		Portal map[string]interface{} `yaml:"dm-portal"`
	}

	// DMTopologySpecification represents the specification of topology.yaml
	DMTopologySpecification struct {
		GlobalOptions    GlobalOptions      `yaml:"global,omitempty"`
		MonitoredOptions MonitoredOptions   `yaml:"monitored,omitempty"`
		ServerConfigs    DMServerConfigs    `yaml:"server_configs,omitempty"`
		Masters          []MasterSpec       `yaml:"dm-master_servers"`
		Workers          []WorkerSpec       `yaml:"dm-worker_servers"`
		Portals          []PortalSpec       `yaml:"dm-portal_servers"`
		Monitors         []PrometheusSpec   `yaml:"monitoring_servers"`
		Grafana          []GrafanaSpec      `yaml:"grafana_servers,omitempty"`
		Alertmanager     []AlertManagerSpec `yaml:"alertmanager_servers,omitempty"`
	}
)

// AllDMComponentNames contains the names of all dm components.
// should include all components in ComponentsByStartOrder
func AllDMComponentNames() (roles []string) {
	tp := &DMSpecification{}
	tp.IterComponent(func(c Component) {
		roles = append(roles, c.Name())
	})

	return
}

// MasterSpec represents the Master topology specification in topology.yaml
type MasterSpec struct {
	Host     string `yaml:"host"`
	SSHPort  int    `yaml:"ssh_port,omitempty"`
	Imported bool   `yaml:"imported,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name            string                 `yaml:"name"`
	Port            int                    `yaml:"port" default:"8261"`
	PeerPort        int                    `yaml:"peer_port" default:"8291"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s MasterSpec) Status(masterList ...string) string {
	if len(masterList) < 1 {
		return "N/A"
	}
	masterapi := api.NewDMMasterClient(masterList, statusQueryTimeout, nil)
	isFound, isActive, isLeader, err := masterapi.GetMaster(s.Name)
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
func (s MasterSpec) Role() string {
	return ComponentDMMaster
}

// SSH returns the host and SSH port of the instance
func (s MasterSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s MasterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s MasterSpec) IsImported() bool {
	return s.Imported
}

// WorkerSpec represents the Master topology specification in topology.yaml
type WorkerSpec struct {
	Host     string `yaml:"host"`
	SSHPort  int    `yaml:"ssh_port,omitempty"`
	Imported bool   `yaml:"imported,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name            string                 `yaml:"name"`
	Port            int                    `yaml:"port" default:"8262"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s WorkerSpec) Status(masterList ...string) string {
	if len(masterList) < 1 {
		return "N/A"
	}
	masterapi := api.NewDMMasterClient(masterList, statusQueryTimeout, nil)
	stage, err := masterapi.GetWorker(s.Name)
	if err != nil {
		return "Down"
	}
	if stage == "" {
		return "N/A"
	}
	return stage
}

// Role returns the component role of the instance
func (s WorkerSpec) Role() string {
	return ComponentDMWorker
}

// SSH returns the host and SSH port of the instance
func (s WorkerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s WorkerSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s WorkerSpec) IsImported() bool {
	return s.Imported
}

// PortalSpec represents the Portal topology specification in topology.yaml
type PortalSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8280"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	Timeout         int                    `yaml:"timeout" default:"5"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s PortalSpec) Role() string {
	return ComponentDMPortal
}

// SSH returns the host and SSH port of the instance
func (s PortalSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s PortalSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s PortalSpec) IsImported() bool {
	return s.Imported
}

// UnmarshalYAML sets default values when unmarshaling the topology file
func (topo *DMTopologySpecification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type topology DMTopologySpecification
	if err := unmarshal((*topology)(topo)); err != nil {
		return err
	}

	if err := defaults.Set(topo); err != nil {
		return errors.Trace(err)
	}

	// Set monitored options
	if topo.MonitoredOptions.DeployDir == "" {
		topo.MonitoredOptions.DeployDir = filepath.Join(topo.GlobalOptions.DeployDir,
			fmt.Sprintf("%s-%d", RoleMonitor, topo.MonitoredOptions.NodeExporterPort))
	}
	if topo.MonitoredOptions.DataDir == "" {
		topo.MonitoredOptions.DataDir = filepath.Join(topo.GlobalOptions.DataDir,
			fmt.Sprintf("%s-%d", RoleMonitor, topo.MonitoredOptions.NodeExporterPort))
	}

	if err := fillDMCustomDefaults(&topo.GlobalOptions, topo); err != nil {
		return err
	}

	return topo.Validate()
}

// platformConflictsDetect checks for conflicts in topology for different OS / Arch
// for set to the same host / IP
func (topo *DMTopologySpecification) platformConflictsDetect() error {
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
			compSpec := compSpecs.Index(index)
			// skip nodes imported from TiDB-Ansible
			if compSpec.Interface().(InstanceSpec).IsImported() {
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
					return errors.Errorf("platform mismatch for '%s' as in '%s:%s/%s' and '%s:%s/%s'",
						host, prev.cfg, prev.os, prev.arch, stat.cfg, stat.os, stat.arch)
				}
			}
			platformStats[host] = stat
		}
	}
	return nil
}

func (topo *DMTopologySpecification) portConflictsDetect() error {
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
			compSpec := compSpecs.Index(index)
			// skip nodes imported from TiDB-Ansible
			if compSpec.Interface().(InstanceSpec).IsImported() {
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
						return errors.Errorf("port '%d' conflicts between '%s:%s.%s' and '%s:%s.%s'",
							item.port, prev.cfg, item.host, prev.tp, cfg, item.host, tp)
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
				return errors.Errorf("port '%d' conflicts between '%s:%s.%s' and '%s:%s.%s'",
					item.port, prev.cfg, item.host, prev.tp, cfg, item.host, tp)
			}
			portStats[item] = conflict{
				tp:  tp,
				cfg: cfg,
			}
		}
	}

	return nil
}

func (topo *DMTopologySpecification) dirConflictsDetect() error {
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
			compSpec := compSpecs.Index(index)
			// skip nodes imported from TiDB-Ansible
			if compSpec.Interface().(InstanceSpec).IsImported() {
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
						return errors.Errorf("directory '%s' conflicts between '%s:%s.%s' and '%s:%s.%s'",
							item.dir, prev.cfg, item.host, prev.tp, cfg, item.host, tp)
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

// Validate validates the topology specification and produce error if the
// specification invalid (e.g: port conflicts or directory conflicts)
func (topo *DMTopologySpecification) Validate() error {
	if err := topo.platformConflictsDetect(); err != nil {
		return err
	}

	if err := topo.portConflictsDetect(); err != nil {
		return err
	}

	return topo.dirConflictsDetect()
}

// GetMasterList returns a list of Master API hosts of the current cluster
func (topo *DMTopologySpecification) GetMasterList() []string {
	var masterList []string

	for _, master := range topo.Masters {
		masterList = append(masterList, fmt.Sprintf("%s:%d", master.Host, master.Port))
	}

	return masterList
}

// Merge returns a new TopologySpecification which sum old ones
func (topo *DMTopologySpecification) Merge(that *DMTopologySpecification) *DMTopologySpecification {
	return &DMTopologySpecification{
		GlobalOptions:    topo.GlobalOptions,
		MonitoredOptions: topo.MonitoredOptions,
		ServerConfigs:    topo.ServerConfigs,
		Masters:          append(topo.Masters, that.Masters...),
		Workers:          append(topo.Workers, that.Workers...),
		Portals:          append(topo.Portals, that.Portals...),
		Monitors:         append(topo.Monitors, that.Monitors...),
		Grafana:          append(topo.Grafana, that.Grafana...),
		Alertmanager:     append(topo.Alertmanager, that.Alertmanager...),
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
					fmt.Sprintf("%s-%s", field.Interface().(InstanceSpec).Role(), getPort(field)),
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
			setDefaultDir(globalOptions.DeployDir, field.Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
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
