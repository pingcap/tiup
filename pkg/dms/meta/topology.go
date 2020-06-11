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
	"time"

	"github.com/creasty/defaults"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	"github.com/pingcap/tiup/pkg/set"
)

const (
	// Timeout in second when quering node status
	statusQueryTimeout = 2 * time.Second
)

type (
	// InstanceSpec represent a instance specification
	InstanceSpec interface {
		Role() string
		SSH() (string, int)
		GetMainPort() int
		IsImported() bool
	}

	// ResourceControl is used to control the system resource
	// See: https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html
	ResourceControl = meta.ResourceControl

	// GlobalOptions represents the global options for all groups in topology
	// specification in topology.yaml
	GlobalOptions = meta.GlobalOptions

	// MonitoredOptions represents the monitored node configuration
	MonitoredOptions = meta.MonitoredOptions

	// DMSServerConfigs represents the server runtime configuration
	DMSServerConfigs struct {
		DMMaster  map[string]interface{} `yaml:"dm-master"`
		DMWorker  map[string]interface{} `yaml:"dm-worker"`
		DMPortal  map[string]interface{} `yaml:"dm-portal"`
		Lightning map[string]interface{} `yaml:"lightning"`
		Importer  map[string]interface{} `yaml:"importer"`
		Dumpling  map[string]interface{} `yaml:"dumpling"`
	}

	// DMSTopologySpecification represents the specification of topology.yaml
	DMSTopologySpecification struct {
		GlobalOptions    GlobalOptions    `yaml:"global,omitempty"`
		MonitoredOptions MonitoredOptions `yaml:"monitored,omitempty"`
		ServerConfigs    DMSServerConfigs `yaml:"server_configs,omitempty"`
		Job              JobSpec          `yaml:"job"`
	}
)

// AllDMSComponentNames contains the names of all dm components.
// should include all components in ComponentsByStartOrder
func AllDMSComponentNames() (roles []string) {
	tp := &DMSSpecification{}
	tp.IterComponent(func(c Component) {
		roles = append(roles, c.Name())
	})

	return
}

// JobSpec represents the data migration job that users want to do
type JobSpec struct {
	Action  string                 `yaml:"action"`
	Type    string                 `yaml:"type"`
	Name    string                 `yaml:"name,omitempty"`
	Config  map[string]interface{} `yaml:"config,omitempty"`
	Sources []SourceSpec           `yaml:"sources"`
	Sink    SourceSpec             `yaml:"sink"`
	Workers []WorkerSpec           `yaml:"workers"`
	// job related specs, once job is stopped, these specs should stopped
	DMMasters    []DMMasterSpec     `yaml:"dm-masters,omitempty"`
	Monitors     []PrometheusSpec   `yaml:"monitoring_servers"`
	Grafana      []GrafanaSpec      `yaml:"grafana_servers,omitempty"`
	Alertmanager []AlertManagerSpec `yaml:"alertmanager_servers,omitempty"`
}

// SourceSpec represents a data source (could be file path or address to a database)
type SourceSpec struct {
	Host     string                 `yaml:"host"`
	Path     string                 `yaml:"path,omitempty"`
	Port     string                 `yaml:"port,omitempty"`
	User     string                 `yaml:"user,omitempty"`
	Password string                 `yaml:"password,omitempty"`
	Config   map[string]interface{} `yaml:"config,omitempty"`
	// source related specs, once source is deleted, these specs should be stopped
	Dumpling  DumplingSpec  `yaml:"dumpling,omitempty"`
	Lightning LightningSpec `yaml:"lightning,omitempty"`
	Importer  ImporterSpec  `yaml:"importer,omitempty"`
	DMWorker  DMWorkerSpec  `yaml:"dm-worker,omitempty"`
}

// WorkerSpec represents a server instance to do the dms job
type WorkerSpec struct {
	Host            string          `yaml:"host"`
	SSHPort         int             `yaml:"ssh_port,omitempty"`
	DeployDir       string          `yaml:"deploy_dir,omitempty"`
	DataDir         string          `yaml:"data_dir,omitempty"`
	LogDir          string          `yaml:"log_dir,omitempty"`
	NumaNode        string          `yaml:"numa_node,omitempty"`
	ResourceControl ResourceControl `yaml:"resource_control,omitempty"`
	Arch            string          `yaml:"arch,omitempty"`
	OS              string          `yaml:"os,omitempty"`
}

// PrometheusSpec represents the Prometheus Server topology specification in topology.yaml
type PrometheusSpec struct {
	Host            string          `yaml:"host"`
	SSHPort         int             `yaml:"ssh_port,omitempty"`
	Imported        bool            `yaml:"imported,omitempty"`
	Port            int             `yaml:"port" default:"9090"`
	DeployDir       string          `yaml:"deploy_dir,omitempty"`
	DataDir         string          `yaml:"data_dir,omitempty"`
	LogDir          string          `yaml:"log_dir,omitempty"`
	NumaNode        string          `yaml:"numa_node,omitempty"`
	Retention       string          `yaml:"storage_retention,omitempty"`
	ResourceControl ResourceControl `yaml:"resource_control,omitempty"`
	Arch            string          `yaml:"arch,omitempty"`
	OS              string          `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s PrometheusSpec) Role() string {
	return ComponentPrometheus
}

// SSH returns the host and SSH port of the instance
func (s PrometheusSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s PrometheusSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s PrometheusSpec) IsImported() bool {
	return s.Imported
}

// GrafanaSpec represents the Grafana topology specification in topology.yaml
type GrafanaSpec struct {
	Host            string          `yaml:"host"`
	SSHPort         int             `yaml:"ssh_port,omitempty"`
	Imported        bool            `yaml:"imported,omitempty"`
	Port            int             `yaml:"port" default:"3000"`
	DeployDir       string          `yaml:"deploy_dir,omitempty"`
	ResourceControl ResourceControl `yaml:"resource_control,omitempty"`
	Arch            string          `yaml:"arch,omitempty"`
	OS              string          `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s GrafanaSpec) Role() string {
	return ComponentGrafana
}

// SSH returns the host and SSH port of the instance
func (s GrafanaSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s GrafanaSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s GrafanaSpec) IsImported() bool {
	return s.Imported
}

// AlertManagerSpec represents the AlertManager topology specification in topology.yaml
type AlertManagerSpec struct {
	Host            string          `yaml:"host"`
	SSHPort         int             `yaml:"ssh_port,omitempty"`
	Imported        bool            `yaml:"imported,omitempty"`
	WebPort         int             `yaml:"web_port" default:"9093"`
	ClusterPort     int             `yaml:"cluster_port" default:"9094"`
	DeployDir       string          `yaml:"deploy_dir,omitempty"`
	DataDir         string          `yaml:"data_dir,omitempty"`
	LogDir          string          `yaml:"log_dir,omitempty"`
	NumaNode        string          `yaml:"numa_node,omitempty"`
	ResourceControl ResourceControl `yaml:"resource_control,omitempty"`
	Arch            string          `yaml:"arch,omitempty"`
	OS              string          `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s AlertManagerSpec) Role() string {
	return ComponentAlertManager
}

// SSH returns the host and SSH port of the instance
func (s AlertManagerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s AlertManagerSpec) GetMainPort() int {
	return s.WebPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s AlertManagerSpec) IsImported() bool {
	return s.Imported
}

// DMMasterSpec represents the DMMaster topology specification in topology.yaml
type DMMasterSpec struct {
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
func (s DMMasterSpec) Status(masterList ...string) string {
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
func (s DMMasterSpec) Role() string {
	return ComponentDMMaster
}

// SSH returns the host and SSH port of the instance
func (s DMMasterSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s DMMasterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s DMMasterSpec) IsImported() bool {
	return s.Imported
}

// DMWorkerSpec represents the DMMaster topology specification in topology.yaml
type DMWorkerSpec struct {
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
func (s DMWorkerSpec) Status(masterList ...string) string {
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
func (s DMWorkerSpec) Role() string {
	return ComponentDMWorker
}

// SSH returns the host and SSH port of the instance
func (s DMWorkerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s DMWorkerSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s DMWorkerSpec) IsImported() bool {
	return s.Imported
}

// PortalSpec represents the DMPortal topology specification in topology.yaml
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

// DumplingSpec represents the DumplingSpec topology specification in topology.yaml
type DumplingSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8280"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s DumplingSpec) Role() string {
	return ComponentDumpling
}

// SSH returns the host and SSH port of the instance
func (s DumplingSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s DumplingSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s DumplingSpec) IsImported() bool {
	return s.Imported
}

// LightningSpec represents the LightningSpec topology specification in topology.yaml
type LightningSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8280"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s LightningSpec) Role() string {
	return ComponentLightning
}

// SSH returns the host and SSH port of the instance
func (s LightningSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s LightningSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s LightningSpec) IsImported() bool {
	return s.Imported
}

// ImporterSpec represents the ImporterSpec topology specification in topology.yaml
type ImporterSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8280"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s ImporterSpec) Role() string {
	return ComponentImporter
}

// SSH returns the host and SSH port of the instance
func (s ImporterSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s ImporterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s ImporterSpec) IsImported() bool {
	return s.Imported
}

// UnmarshalYAML sets default values when unmarshaling the topology file
func (topo *DMSTopologySpecification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type topology DMSTopologySpecification
	if err := unmarshal((*topology)(topo)); err != nil {
		return err
	}

	if err := defaults.Set(topo); err != nil {
		return errors.Trace(err)
	}

	// Set monitored options
	if topo.MonitoredOptions.DeployDir == "" {
		topo.MonitoredOptions.DeployDir = filepath.Join(topo.GlobalOptions.DeployDir,
			fmt.Sprintf("%s-%d", meta.RoleMonitor, topo.MonitoredOptions.NodeExporterPort))
	}
	if topo.MonitoredOptions.DataDir == "" {
		topo.MonitoredOptions.DataDir = filepath.Join(topo.GlobalOptions.DataDir,
			fmt.Sprintf("%s-%d", meta.RoleMonitor, topo.MonitoredOptions.NodeExporterPort))
	}

	if err := fillDMCustomDefaults(&topo.GlobalOptions, topo); err != nil {
		return err
	}

	return topo.Validate()
}

var (
	monitorOptionTypeName   = reflect.TypeOf(MonitoredOptions{}).Name()
	dmServerConfigsTypeName = reflect.TypeOf(DMSServerConfigs{}).Name()
)

// Skip global/monitored options
func isSkipField(field reflect.Value) bool {
	tp := field.Type().Name()
	return tp == dmServerConfigsTypeName
}

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

// platformConflictsDetect checks for conflicts in topology for different OS / Arch
// for set to the same host / IP
func (topo *DMSTopologySpecification) platformConflictsDetect() error {
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

func (topo *DMSTopologySpecification) portConflictsDetect() error {
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

func (topo *DMSTopologySpecification) dirConflictsDetect() error {
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
func (topo *DMSTopologySpecification) Validate() error {
	if err := topo.platformConflictsDetect(); err != nil {
		return err
	}

	if err := topo.portConflictsDetect(); err != nil {
		return err
	}

	return topo.dirConflictsDetect()
}

// GetMasterList returns a list of DMMaster API hosts of the current cluster
func (topo *DMSTopologySpecification) GetMasterList() []string {
	var masterList []string

	//for _, master := range topo.Masters {
	//	masterList = append(masterList, fmt.Sprintf("%s:%d", master.Host, master.Port))
	//}

	return masterList
}

// Merge returns a new TopologySpecification which sum old ones
func (topo *DMSTopologySpecification) Merge(that *DMSTopologySpecification) *DMSTopologySpecification {
	return &DMSTopologySpecification{
		GlobalOptions:    topo.GlobalOptions,
		MonitoredOptions: topo.MonitoredOptions,
		ServerConfigs:    topo.ServerConfigs,
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

func getPort(v reflect.Value) string {
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "Port", "ClientPort", "WebPort", "TCPPort", "NodeExporterPort":
			return fmt.Sprintf("%d", v.Field(i).Int())
		}
	}
	return ""
}
