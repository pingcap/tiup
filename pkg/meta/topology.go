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
	"time"

	"github.com/creasty/defaults"
	"github.com/pingcap-incubator/tiops/pkg/api"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap/errors"
)

const (
	// Timeout in second when quering node status
	statusQueryTimeout = 2 * time.Second
)

// Roles of components
const (
	RoleTiDB    = "tidb"
	RoleTiKV    = "tikv"
	RolePD      = "pd"
	RoleGrafana = "grafana"
	RoleDrainer = "drainer"
	RolePump    = "pump"
	RoleMonitor = "monitor"
)

type (
	// InstanceSpec represent a instance specification
	InstanceSpec interface {
		Role() string
	}

	// GlobalOptions represents the global options for all groups in topology
	// pecification in topology.yaml
	GlobalOptions struct {
		User      string `yaml:"user,omitempty" default:"tidb"`
		SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
		DeployDir string `yaml:"deploy_dir,omitempty" default:"deploy"`
		DataDir   string `yaml:"data_dir,omitempty"  default:"data"`
	}

	// MonitoredOptions represents the monitored node configuration
	MonitoredOptions struct {
		NodeExporterPort     int    `yaml:"node_exporter_port,omitempty" default:"9100"`
		BlackboxExporterPort int    `yaml:"blackbox_exporter_port,omitempty" default:"9115"`
		DeployDir            string `yaml:"deploy_dir,omitempty"`
		DataDir              string `yaml:"data_dir,omitempty"`
	}

	// TopologySpecification represents the specification of topology.yaml
	TopologySpecification struct {
		GlobalOptions    GlobalOptions      `yaml:"global,omitempty"`
		MonitoredOptions MonitoredOptions   `yaml:"monitored,omitempty"`
		TiDBServers      []TiDBSpec         `yaml:"tidb_servers"`
		TiKVServers      []TiKVSpec         `yaml:"tikv_servers"`
		PDServers        []PDSpec           `yaml:"pd_servers"`
		PumpServers      []PumpSpec         `yaml:"pump_servers,omitempty"`
		Drainers         []DrainerSpec      `yaml:"drainer_servers,omitempty"`
		Monitors         []PrometheusSpec   `yaml:"monitoring_servers"`
		Grafana          []GrafanaSpec      `yaml:"grafana_servers,omitempty"`
		Alertmanager     []AlertManagerSpec `yaml:"alertmanager_servers,omitempty"`
	}
)

// TiDBSpec represents the TiDB topology specification in topology.yaml
type TiDBSpec struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port" default:"4000"`
	StatusPort int    `yaml:"status_port" default:"10080"`
	SSHPort    int    `yaml:"ssh_port,omitempty"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	NumaNode   bool   `yaml:"numa_node,omitempty"`
}

// Status queries current status of the instance
func (s TiDBSpec) Status(pdList ...string) string {
	client := utils.NewHTTPClient(statusQueryTimeout, nil)
	url := fmt.Sprintf("http://%s:%d/status", s.Host, s.StatusPort)

	// body doesn't have any status section needed
	body, err := client.Get(url)
	if err != nil {
		return "ERR"
	}
	if body == nil {
		return "Down"
	}
	return "Up"
}

// Role returns the component role of the instance
func (s TiDBSpec) Role() string {
	return RoleTiDB
}

// TiKVSpec represents the TiKV topology specification in topology.yaml
type TiKVSpec struct {
	Host       string   `yaml:"host"`
	Port       int      `yaml:"port" default:"20160"`
	StatusPort int      `yaml:"status_port" default:"20180"`
	SSHPort    int      `yaml:"ssh_port,omitempty"`
	DeployDir  string   `yaml:"deploy_dir,omitempty"`
	DataDir    string   `yaml:"data_dir,omitempty"`
	Offline    bool     `yaml:"offline,omitempty"`
	Labels     []string `yaml:"labels,omitempty"`
	NumaNode   bool     `yaml:"numa_node,omitempty"`
}

// Status queries current status of the instance
func (s TiKVSpec) Status(pdList ...string) string {
	if len(pdList) < 1 {
		return "N/A"
	}
	pdapi := api.NewPDClient(pdList[0], statusQueryTimeout, nil)
	stores, err := pdapi.GetStores()
	if err != nil {
		return "ERR"
	}

	name := fmt.Sprintf("%s:%d", s.Host, s.Port)
	for _, store := range stores.Stores {
		if name == store.Store.Address {
			return store.Store.StateName
		}
	}
	return "N/A"
}

// Role returns the component role of the instance
func (s TiKVSpec) Role() string {
	return RoleTiKV
}

// PDSpec represents the PD topology specification in topology.yaml
type PDSpec struct {
	// Use Name to get the name with a default value if it's empty.
	Name       string `yaml:"name"`
	Host       string `yaml:"host"`
	ClientPort int    `yaml:"client_port" default:"2379"`
	PeerPort   int    `yaml:"peer_port" default:"2380"`
	SSHPort    int    `yaml:"ssh_port,omitempty"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	DataDir    string `yaml:"data_dir,omitempty"`
	NumaNode   bool   `yaml:"numa_node,omitempty"`
}

// Status queries current status of the instance
func (s PDSpec) Status(pdList ...string) string {
	pdapi := api.NewPDClient(fmt.Sprintf("%s:%d", s.Host, s.ClientPort),
		statusQueryTimeout, nil)
	healths, err := pdapi.GetHealth()
	if err != nil {
		return "ERR"
	}

	// find leader node
	leader, err := pdapi.GetLeader()
	if err != nil {
		return "ERR"
	}

	for _, member := range healths.Healths {
		suffix := ""
		if s.Name != member.Name {
			continue
		}
		if s.Name == leader.Name {
			suffix = "|L"
		}
		if member.Health {
			return "Healthy" + suffix
		}
		return "Unhealthy"
	}
	return "N/A"
}

// Role returns the component role of the instance
func (s PDSpec) Role() string {
	return RolePD
}

// PumpSpec represents the Pump topology specification in topology.yaml
type PumpSpec struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port" default:"8250"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
	Offline   bool   `yaml:"offline,omitempty"`
	NumaNode  bool   `yaml:"numa_node,omitempty"`
}

// Role returns the component role of the instance
func (s PumpSpec) Role() string {
	return RolePump
}

// DrainerSpec represents the Drainer topology specification in topology.yaml
type DrainerSpec struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port" default:"8249"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
	CommitTS  string `yaml:"commit_ts,omitempty"`
	Offline   bool   `yaml:"offline,omitempty"`
	NumaNode  bool   `yaml:"numa_node,omitempty"`
}

// Role returns the component role of the instance
func (s DrainerSpec) Role() string {
	return RoleDrainer
}

// PrometheusSpec represents the Prometheus Server topology specification in topology.yaml
type PrometheusSpec struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port" default:"9090"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
}

// Role returns the component role of the instance
func (s PrometheusSpec) Role() string {
	return RoleMonitor
}

// GrafanaSpec represents the Grafana topology specification in topology.yaml
type GrafanaSpec struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port" default:"3000"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
}

// Role returns the component role of the instance
func (s GrafanaSpec) Role() string {
	return RoleMonitor
}

// AlertManagerSpec represents the AlertManager topology specification in topology.yaml
type AlertManagerSpec struct {
	Host        string `yaml:"host"`
	WebPort     int    `yaml:"web_port" default:"9093"`
	ClusterPort int    `yaml:"cluster_port" default:"9094"`
	SSHPort     int    `yaml:"ssh_port,omitempty"`
	DeployDir   string `yaml:"deploy_dir,omitempty"`
	DataDir     string `yaml:"data_dir,omitempty"`
}

// Role returns the component role of the instance
func (s AlertManagerSpec) Role() string {
	return RoleMonitor
}

// UnmarshalYAML sets default values when unmarshaling the topology file
func (topo *TopologySpecification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type topology TopologySpecification
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

	if err := fillCustomDefaults(&topo.GlobalOptions, topo); err != nil {
		return err
	}

	return nil
}

// GetPDList returns a list of PD API hosts of the current cluster
func (topo *TopologySpecification) GetPDList() []string {
	var pdList []string

	for _, pd := range topo.PDServers {
		pdList = append(pdList, fmt.Sprintf("%s:%d", pd.Host, pd.ClientPort))
	}

	return pdList
}

// Merge returns a new TopologySpecification which sum old ones
func (topo *TopologySpecification) Merge(that *TopologySpecification) *TopologySpecification {
	return &TopologySpecification{
		TiDBServers:  append(topo.TiDBServers, that.TiDBServers...),
		TiKVServers:  append(topo.TiKVServers, that.TiKVServers...),
		PDServers:    append(topo.PDServers, that.PDServers...),
		PumpServers:  append(topo.PumpServers, that.PumpServers...),
		Drainers:     append(topo.Drainers, that.Drainers...),
		Monitors:     append(topo.Monitors, that.Monitors...),
		Grafana:      append(topo.Grafana, that.Grafana...),
		Alertmanager: append(topo.Alertmanager, that.Alertmanager...),
	}
}

// Validate validates the topology specification and produce error if
// the specification invalid
func (topo *TopologySpecification) Validate() error {
	return nil
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
	globalOptionTypeName  = reflect.TypeOf(GlobalOptions{}).Name()
	monitorOptionTypeName = reflect.TypeOf(MonitoredOptions{}).Name()
)

// Skip global/monitored options
func isSkipField(field reflect.Value) bool {
	tp := field.Type().Name()
	return tp == globalOptionTypeName || tp == monitorOptionTypeName
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
			host := field.FieldByName("Host").Interface().(string)
			clientPort := field.FieldByName("ClientPort").Interface().(int)
			field.Field(j).Set(reflect.ValueOf(fmt.Sprintf("pd-%s-%d", host, clientPort)))
		case "DataDir":
			setDefaultDir(globalOptions.DataDir, field.Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
		case "DeployDir":
			setDefaultDir(globalOptions.DeployDir, field.Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
		}
	}

	return nil
}

func getPort(v reflect.Value) string {
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "Port", "ClientPort", "WebPort", "NodeExporterPort":
			return fmt.Sprintf("%d", v.Field(i).Int())
		}
	}
	return ""
}
