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

	utils2 "github.com/pingcap/tiup/pkg/utils"

	"go.etcd.io/etcd/clientv3"

	"github.com/creasty/defaults"
	"github.com/pingcap/errors"
	pdserverapi "github.com/pingcap/pd/v4/server/api"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/set"
)

const (
	// Timeout in second when quering node status
	statusQueryTimeout = 10 * time.Second
)

// general role names
var (
	RoleMonitor = "monitor"
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
	ResourceControl struct {
		MemoryLimit         string `yaml:"memory_limit,omitempty"`
		CPUQuota            string `yaml:"cpu_quota,omitempty"`
		IOReadBandwidthMax  string `yaml:"io_read_bandwidth_max,omitempty"`
		IOWriteBandwidthMax string `yaml:"io_write_bandwidth_max,omitempty"`
	}

	// GlobalOptions represents the global options for all groups in topology
	// specification in topology.yaml
	GlobalOptions struct {
		User            string          `yaml:"user,omitempty" default:"tidb"`
		SSHPort         int             `yaml:"ssh_port,omitempty" default:"22"`
		DeployDir       string          `yaml:"deploy_dir,omitempty" default:"deploy"`
		DataDir         string          `yaml:"data_dir,omitempty" default:"data"`
		LogDir          string          `yaml:"log_dir,omitempty"`
		ResourceControl ResourceControl `yaml:"resource_control,omitempty"`
		OS              string          `yaml:"os,omitempty" default:"linux"`
		Arch            string          `yaml:"arch,omitempty" default:"amd64"`
	}

	// MonitoredOptions represents the monitored node configuration
	MonitoredOptions struct {
		NodeExporterPort     int             `yaml:"node_exporter_port,omitempty" default:"9100"`
		BlackboxExporterPort int             `yaml:"blackbox_exporter_port,omitempty" default:"9115"`
		DeployDir            string          `yaml:"deploy_dir,omitempty"`
		DataDir              string          `yaml:"data_dir,omitempty"`
		LogDir               string          `yaml:"log_dir,omitempty"`
		NumaNode             string          `yaml:"numa_node,omitempty"`
		ResourceControl      ResourceControl `yaml:"resource_control,omitempty"`
	}

	// ServerConfigs represents the server runtime configuration
	ServerConfigs struct {
		TiDB           map[string]interface{} `yaml:"tidb"`
		TiKV           map[string]interface{} `yaml:"tikv"`
		PD             map[string]interface{} `yaml:"pd"`
		TiFlash        map[string]interface{} `yaml:"tiflash"`
		TiFlashLearner map[string]interface{} `yaml:"tiflash-learner"`
		Pump           map[string]interface{} `yaml:"pump"`
		Drainer        map[string]interface{} `yaml:"drainer"`
		CDC            map[string]interface{} `yaml:"cdc"`
	}

	// ClusterSpecification represents the specification of topology.yaml
	ClusterSpecification struct {
		GlobalOptions    GlobalOptions      `yaml:"global,omitempty"`
		MonitoredOptions MonitoredOptions   `yaml:"monitored,omitempty"`
		ServerConfigs    ServerConfigs      `yaml:"server_configs,omitempty"`
		TiDBServers      []TiDBSpec         `yaml:"tidb_servers"`
		TiKVServers      []TiKVSpec         `yaml:"tikv_servers"`
		TiFlashServers   []TiFlashSpec      `yaml:"tiflash_servers"`
		PDServers        []PDSpec           `yaml:"pd_servers"`
		PumpServers      []PumpSpec         `yaml:"pump_servers,omitempty"`
		Drainers         []DrainerSpec      `yaml:"drainer_servers,omitempty"`
		CDCServers       []CDCSpec          `yaml:"cdc_servers,omitempty"`
		Monitors         []PrometheusSpec   `yaml:"monitoring_servers"`
		Grafana          []GrafanaSpec      `yaml:"grafana_servers,omitempty"`
		Alertmanager     []AlertManagerSpec `yaml:"alertmanager_servers,omitempty"`
	}
)

// AllComponentNames contains the names of all components.
// should include all components in ComponentsByStartOrder
func AllComponentNames() (roles []string) {
	tp := &ClusterSpecification{}
	tp.IterComponent(func(c Component) {
		roles = append(roles, c.Name())
	})

	return
}

// TiDBSpec represents the TiDB topology specification in topology.yaml
type TiDBSpec struct {
	Host            string                 `yaml:"host"`
	ListenHost      string                 `yaml:"listen_host,omitempty"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"4000"`
	StatusPort      int                    `yaml:"status_port" default:"10080"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// statusByURL queries current status of the instance by http status api.
func statusByURL(url string) string {
	client := utils2.NewHTTPClient(statusQueryTimeout, nil)

	// body doesn't have any status section needed
	body, err := client.Get(url)
	if err != nil {
		return "Down"
	}
	if body == nil {
		return "Down"
	}
	return "Up"

}

// Status queries current status of the instance
func (s TiDBSpec) Status(pdList ...string) string {
	url := fmt.Sprintf("http://%s:%d/status", s.Host, s.StatusPort)
	return statusByURL(url)
}

// Role returns the component role of the instance
func (s TiDBSpec) Role() string {
	return ComponentTiDB
}

// SSH returns the host and SSH port of the instance
func (s TiDBSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiDBSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiDBSpec) IsImported() bool {
	return s.Imported
}

// TiKVSpec represents the TiKV topology specification in topology.yaml
type TiKVSpec struct {
	Host            string                 `yaml:"host"`
	ListenHost      string                 `yaml:"listen_host,omitempty"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"20160"`
	StatusPort      int                    `yaml:"status_port" default:"20180"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	Offline         bool                   `yaml:"offline,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// checkStoreStatus checks the store status in current cluster
func checkStoreStatus(storeAddr string, pdList ...string) string {
	if len(pdList) < 1 {
		return "N/A"
	}
	pdapi := api.NewPDClient(pdList, statusQueryTimeout, nil)
	stores, err := pdapi.GetStores()
	if err != nil {
		return "Down"
	}

	// only get status of the latest store, it is the store with lagest ID number
	// older stores might be legacy ones that already offlined
	var latestStore *pdserverapi.StoreInfo
	for _, store := range stores.Stores {
		if storeAddr == store.Store.Address {
			if latestStore == nil {
				latestStore = store
				continue
			}
			if store.Store.Id > latestStore.Store.Id {
				latestStore = store
			}
		}
	}
	if latestStore != nil {
		return latestStore.Store.StateName
	}
	return "N/A"
}

// Status queries current status of the instance
func (s TiKVSpec) Status(pdList ...string) string {
	storeAddr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	state := checkStoreStatus(storeAddr, pdList...)
	if s.Offline && strings.ToLower(state) == "offline" {
		state = "Pending Offline" // avoid misleading
	}
	return state
}

// Role returns the component role of the instance
func (s TiKVSpec) Role() string {
	return ComponentTiKV
}

// SSH returns the host and SSH port of the instance
func (s TiKVSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiKVSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiKVSpec) IsImported() bool {
	return s.Imported
}

// PDSpec represents the PD topology specification in topology.yaml
type PDSpec struct {
	Host       string `yaml:"host"`
	ListenHost string `yaml:"listen_host,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty"`
	Imported   bool   `yaml:"imported,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name            string                 `yaml:"name"`
	ClientPort      int                    `yaml:"client_port" default:"2379"`
	PeerPort        int                    `yaml:"peer_port" default:"2380"`
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
func (s PDSpec) Status(pdList ...string) string {
	curAddr := fmt.Sprintf("%s:%d", s.Host, s.ClientPort)
	curPdAPI := api.NewPDClient([]string{curAddr}, statusQueryTimeout, nil)
	allPdAPI := api.NewPDClient(pdList, statusQueryTimeout, nil)
	suffix := ""

	// find dashboard node
	dashboardAddr, _ := allPdAPI.GetDashboardAddress()
	if strings.HasPrefix(dashboardAddr, "http") {
		r := strings.NewReplacer("http://", "", "https://", "")
		dashboardAddr = r.Replace(dashboardAddr)
	}
	if dashboardAddr == curAddr {
		suffix = "|UI"
	}

	// check health
	err := curPdAPI.CheckHealth()
	if err != nil {
		return "Down" + suffix
	}

	// find leader node
	leader, err := curPdAPI.GetLeader()
	if err != nil {
		return "ERR" + suffix
	}
	if s.Name == leader.Name {
		suffix = "|L" + suffix
	}
	return "Up" + suffix
}

// Role returns the component role of the instance
func (s PDSpec) Role() string {
	return ComponentPD
}

// SSH returns the host and SSH port of the instance
func (s PDSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s PDSpec) GetMainPort() int {
	return s.ClientPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s PDSpec) IsImported() bool {
	return s.Imported
}

// TiFlashSpec represents the TiFlash topology specification in topology.yaml
type TiFlashSpec struct {
	Host                 string                 `yaml:"host"`
	SSHPort              int                    `yaml:"ssh_port,omitempty"`
	Imported             bool                   `yaml:"imported,omitempty"`
	TCPPort              int                    `yaml:"tcp_port" default:"9000"`
	HTTPPort             int                    `yaml:"http_port" default:"8123"`
	FlashServicePort     int                    `yaml:"flash_service_port" default:"3930"`
	FlashProxyPort       int                    `yaml:"flash_proxy_port" default:"20170"`
	FlashProxyStatusPort int                    `yaml:"flash_proxy_status_port" default:"20292"`
	StatusPort           int                    `yaml:"metrics_port" default:"8234"`
	DeployDir            string                 `yaml:"deploy_dir,omitempty"`
	DataDir              string                 `yaml:"data_dir,omitempty"`
	LogDir               string                 `yaml:"log_dir,omitempty"`
	TmpDir               string                 `yaml:"tmp_path,omitempty"`
	Offline              bool                   `yaml:"offline,omitempty"`
	NumaNode             string                 `yaml:"numa_node,omitempty"`
	Config               map[string]interface{} `yaml:"config,omitempty"`
	LearnerConfig        map[string]interface{} `yaml:"learner_config,omitempty"`
	ResourceControl      ResourceControl        `yaml:"resource_control,omitempty"`
	Arch                 string                 `yaml:"arch,omitempty"`
	OS                   string                 `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s TiFlashSpec) Status(pdList ...string) string {
	storeAddr := fmt.Sprintf("%s:%d", s.Host, s.FlashServicePort)
	state := checkStoreStatus(storeAddr, pdList...)
	if s.Offline && strings.ToLower(state) == "offline" {
		state = "Pending Offline" // avoid misleading
	}
	return state
}

// Role returns the component role of the instance
func (s TiFlashSpec) Role() string {
	return ComponentTiFlash
}

// SSH returns the host and SSH port of the instance
func (s TiFlashSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiFlashSpec) GetMainPort() int {
	return s.TCPPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiFlashSpec) IsImported() bool {
	return s.Imported
}

// PumpSpec represents the Pump topology specification in topology.yaml
type PumpSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8250"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	Offline         bool                   `yaml:"offline,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s PumpSpec) Role() string {
	return ComponentPump
}

// SSH returns the host and SSH port of the instance
func (s PumpSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s PumpSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s PumpSpec) IsImported() bool {
	return s.Imported
}

// DrainerSpec represents the Drainer topology specification in topology.yaml
type DrainerSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8249"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	CommitTS        int64                  `yaml:"commit_ts,omitempty"`
	Offline         bool                   `yaml:"offline,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s DrainerSpec) Role() string {
	return ComponentDrainer
}

// SSH returns the host and SSH port of the instance
func (s DrainerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s DrainerSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s DrainerSpec) IsImported() bool {
	return s.Imported
}

// CDCSpec represents the Drainer topology specification in topology.yaml
type CDCSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"8300"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	Offline         bool                   `yaml:"offline,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty"`
	Config          map[string]interface{} `yaml:"config,omitempty"`
	ResourceControl ResourceControl        `yaml:"resource_control,omitempty"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s CDCSpec) Role() string {
	return ComponentCDC
}

// SSH returns the host and SSH port of the instance
func (s CDCSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s CDCSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s CDCSpec) IsImported() bool {
	return s.Imported
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

// UnmarshalYAML sets default values when unmarshaling the topology file
func (topo *ClusterSpecification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type topology ClusterSpecification
	if err := unmarshal((*topology)(topo)); err != nil {
		return err
	}

	// set default values from tag
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
	if topo.MonitoredOptions.LogDir == "" {
		topo.MonitoredOptions.LogDir = "log"
	}
	if !strings.HasPrefix(topo.MonitoredOptions.LogDir, "/") &&
		!strings.HasPrefix(topo.MonitoredOptions.LogDir, topo.MonitoredOptions.DeployDir) {
		topo.MonitoredOptions.LogDir = filepath.Join(topo.MonitoredOptions.DeployDir, topo.MonitoredOptions.LogDir)
	}

	// populate custom default values as needed
	if err := fillCustomDefaults(&topo.GlobalOptions, topo); err != nil {
		return err
	}

	return topo.Validate()
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
// set to the same host / IP
func (topo *ClusterSpecification) platformConflictsDetect() error {
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
					return &ValidateErr{
						ty:     errTypeMismatch,
						target: "platform",
						one:    fmt.Sprintf("%s:%s/%s", prev.cfg, prev.os, prev.arch),
						two:    fmt.Sprintf("%s:%s/%s", stat.cfg, stat.os, stat.arch),
						value:  host,
					}
				}
			}
			platformStats[host] = stat
		}
	}
	return nil
}

func (topo *ClusterSpecification) portConflictsDetect() error {
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
						return &ValidateErr{
							ty:     errTypeConflict,
							target: "port",
							one:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
							two:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
							value:  item.port,
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
				return &ValidateErr{
					ty:     errTypeConflict,
					target: "port",
					one:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
					two:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
					value:  item.port,
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

func (topo *ClusterSpecification) dirConflictsDetect() error {
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
						return &ValidateErr{
							ty:     errTypeConflict,
							target: "directory",
							one:    fmt.Sprintf("%s:%s.%s", prev.cfg, item.host, prev.tp),
							two:    fmt.Sprintf("%s:%s.%s", cfg, item.host, tp),
							value:  item.dir,
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
func (topo *ClusterSpecification) CountDir(targetHost, dirPrefix string) int {
	dirTypes := []string{
		"DataDir",
		"DeployDir",
		"LogDir",
	}

	// path -> count
	dirStats := make(map[string]int)
	count := 0
	topoSpec := reflect.ValueOf(topo).Elem()

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

					if dir == "" {
						continue
					}
					if !strings.HasPrefix(dir, "/") {
						switch dirType {
						case "DataDir":
							if !strings.HasPrefix(dir, topo.GlobalOptions.DataDir+"/") {
								dir = filepath.Join(topo.GlobalOptions.DataDir, dir)
							}
						case "DeployDir":
							if !strings.HasPrefix(dir, topo.GlobalOptions.DeployDir+"/") {
								dir = filepath.Join(topo.GlobalOptions.DeployDir, dir)
							}
						case "LogDir":
							if !strings.HasPrefix(dir, topo.GlobalOptions.LogDir+"/") {
								dir = filepath.Join(topo.GlobalOptions.LogDir, dir)
							}

						}
					}
					dirStats[host+dir] += 1
				}
			}
		}
	}

	for k, v := range dirStats {
		if strings.HasPrefix(k, targetHost+dirPrefix) {
			count += v
		}
	}

	return count
}

// Validate validates the topology specification and produce error if the
// specification invalid (e.g: port conflicts or directory conflicts)
func (topo *ClusterSpecification) Validate() error {
	if err := topo.platformConflictsDetect(); err != nil {
		return err
	}

	if err := topo.portConflictsDetect(); err != nil {
		return err
	}

	return topo.dirConflictsDetect()
}

// GetPDList returns a list of PD API hosts of the current cluster
func (topo *ClusterSpecification) GetPDList() []string {
	var pdList []string

	for _, pd := range topo.PDServers {
		pdList = append(pdList, fmt.Sprintf("%s:%d", pd.Host, pd.ClientPort))
	}

	return pdList
}

// GetEtcdClient load EtcdClient of current cluster
func (topo *ClusterSpecification) GetEtcdClient() (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints: topo.GetPDList(),
	})
}

// Merge returns a new ClusterSpecification which sum old ones
func (topo *ClusterSpecification) Merge(that *ClusterSpecification) *ClusterSpecification {
	return &ClusterSpecification{
		GlobalOptions:    topo.GlobalOptions,
		MonitoredOptions: topo.MonitoredOptions,
		ServerConfigs:    topo.ServerConfigs,
		TiDBServers:      append(topo.TiDBServers, that.TiDBServers...),
		TiKVServers:      append(topo.TiKVServers, that.TiKVServers...),
		PDServers:        append(topo.PDServers, that.PDServers...),
		TiFlashServers:   append(topo.TiFlashServers, that.TiFlashServers...),
		PumpServers:      append(topo.PumpServers, that.PumpServers...),
		Drainers:         append(topo.Drainers, that.Drainers...),
		CDCServers:       append(topo.CDCServers, that.CDCServers...),
		Monitors:         append(topo.Monitors, that.Monitors...),
		Grafana:          append(topo.Grafana, that.Grafana...),
		Alertmanager:     append(topo.Alertmanager, that.Alertmanager...),
	}
}

// fillDefaults tries to fill custom fields to their default values
func fillCustomDefaults(globalOptions *GlobalOptions, data interface{}) error {
	v := reflect.ValueOf(data).Elem()
	t := v.Type()

	var err error
	for i := 0; i < t.NumField(); i++ {
		if err = setCustomDefaults(globalOptions, v.Field(i)); err != nil {
			return err
		}
	}

	return nil
}

var (
	globalOptionTypeName    = reflect.TypeOf(GlobalOptions{}).Name()
	monitorOptionTypeName   = reflect.TypeOf(MonitoredOptions{}).Name()
	serverConfigsTypeName   = reflect.TypeOf(ServerConfigs{}).Name()
	dmServerConfigsTypeName = reflect.TypeOf(DMServerConfigs{}).Name()
)

// Skip global/monitored options
func isSkipField(field reflect.Value) bool {
	tp := field.Type().Name()
	return tp == globalOptionTypeName || tp == monitorOptionTypeName || tp == serverConfigsTypeName || tp == dmServerConfigsTypeName
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

func setCustomDefaults(globalOptions *GlobalOptions, field reflect.Value) error {
	if !field.CanSet() || isSkipField(field) {
		return nil
	}

	switch field.Kind() {
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			if err := setCustomDefaults(globalOptions, field.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		ref := reflect.New(field.Type())
		ref.Elem().Set(field)
		if err := fillCustomDefaults(globalOptions, ref.Interface()); err != nil {
			return err
		}
		field.Set(ref.Elem())
	case reflect.Ptr:
		if err := setCustomDefaults(globalOptions, field.Elem()); err != nil {
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
			clientPort := field.FieldByName("ClientPort").Int()
			field.Field(j).Set(reflect.ValueOf(fmt.Sprintf("pd-%s-%d", host, clientPort)))
		case "DataDir":
			if field.FieldByName("Imported").Interface().(bool) {
				setDefaultDir(globalOptions.DataDir, field.Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
			}

			dataDir := field.Field(j).String()
			if dataDir != "" { // already have a value, skip filling default values
				continue
			}
			// If the data dir in global options is an absolute path, it appends to
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
