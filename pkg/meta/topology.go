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
	"reflect"
	"time"

	"github.com/creasty/defaults"
	"github.com/pingcap-incubator/tiops/pkg/api"
	"github.com/pingcap-incubator/tiops/pkg/utils"
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

// InstanceSpec is the interface for any instance
type InstanceSpec interface {
	GetID() string
	GetHost() string
	GetPort() []int
	GetSSHPort() int
	GetDir() []string
	GetStatus(pdList ...string) string
	Role() string
}

// TiDBSpec represents the TiDB topology specification in topology.yaml
type TiDBSpec struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port" default:"4000"`
	StatusPort int    `yaml:"status_port" default:"10080"`
	UUID       string `yaml:"uuid,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	NumaNode   bool   `yaml:"numa_node,omitempty"`
}

// GetID returns the UUID of the instance
func (s TiDBSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s TiDBSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s TiDBSpec) GetPort() []int {
	return []int{
		s.Port,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s TiDBSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s TiDBSpec) GetDir() []string {
	return []string{
		s.DeployDir,
	}
}

// GetStatus queries current status of the instance
func (s TiDBSpec) GetStatus(pdList ...string) string {
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
	UUID       string   `yaml:"uuid,omitempty"`
	SSHPort    int      `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir  string   `yaml:"deploy_dir,omitempty"`
	DataDir    string   `yaml:"data_dir,omitempty"`
	Offline    bool     `yaml:"offline,omitempty"`
	Labels     []string `yaml:"labels,omitempty"`
	NumaNode   bool     `yaml:"numa_node,omitempty"`
}

// GetID returns the UUID of the instance
func (s TiKVSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s TiKVSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s TiKVSpec) GetPort() []int {
	return []int{
		s.Port,
		s.StatusPort,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s TiKVSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s TiKVSpec) GetDir() []string {
	return []string{
		s.DeployDir,
		s.DataDir,
	}
}

// GetStatus queries current status of the instance
func (s TiKVSpec) GetStatus(pdList ...string) string {
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
	// Use GetName() to get the name with a default value if it's empty.
	ConfigName string `yaml:"name"`
	Host       string `yaml:"host"`
	ClientPort int    `yaml:"client_port" default:"2379"`
	PeerPort   int    `yaml:"peer_port" default:"2380"`
	UUID       string `yaml:"uuid,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	DataDir    string `yaml:"data_dir,omitempty"`
	NumaNode   bool   `yaml:"numa_node,omitempty"`
}

// GetName return the name.
// return "pd-{host}-{port} if it's not setted.
func (s PDSpec) GetName() string {
	if s.ConfigName != "" {
		return s.ConfigName
	}

	return fmt.Sprintf("pd-%s-%d", s.Host, s.ClientPort)
}

// GetID returns the UUID of the instance
func (s PDSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s PDSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s PDSpec) GetPort() []int {
	return []int{
		s.ClientPort,
		s.PeerPort,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s PDSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s PDSpec) GetDir() []string {
	return []string{
		s.DeployDir,
		s.DataDir,
	}
}

// GetStatus queries current status of the instance
func (s PDSpec) GetStatus(pdList ...string) string {
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
		if s.UUID != member.Name {
			continue
		}
		if s.UUID == leader.Name {
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
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
	Offline   bool   `yaml:"offline,omitempty"`
	NumaNode  bool   `yaml:"numa_node,omitempty"`
}

// GetID returns the UUID of the instance
func (s PumpSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s PumpSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s PumpSpec) GetPort() []int {
	return []int{
		s.Port,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s PumpSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s PumpSpec) GetDir() []string {
	return []string{
		s.DeployDir,
		s.DataDir,
	}
}

// GetStatus queries current status of the instance
func (s PumpSpec) GetStatus(pdList ...string) string {
	return "N/A"
}

// Role returns the component role of the instance
func (s PumpSpec) Role() string {
	return RolePump
}

// DrainerSpec represents the Drainer topology specification in topology.yaml
type DrainerSpec struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port" default:"8249"`
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
	CommitTS  string `yaml:"commit_ts,omitempty"`
	Offline   bool   `yaml:"offline,omitempty"`
	NumaNode  bool   `yaml:"numa_node,omitempty"`
}

// GetID returns the UUID of the instance
func (s DrainerSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s DrainerSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s DrainerSpec) GetPort() []int {
	return []int{
		s.Port,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s DrainerSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s DrainerSpec) GetDir() []string {
	return []string{
		s.DeployDir,
		s.DataDir,
	}
}

// GetStatus queries current status of the instance
func (s DrainerSpec) GetStatus(pdList ...string) string {
	return "N/A"
}

// Role returns the component role of the instance
func (s DrainerSpec) Role() string {
	return RoleDrainer
}

// PrometheusSpec represents the Prometheus Server topology specification in topology.yaml
type PrometheusSpec struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port" default:"9090"`
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
}

// GetID returns the UUID of the instance
func (s PrometheusSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s PrometheusSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s PrometheusSpec) GetPort() []int {
	return []int{
		s.Port,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s PrometheusSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s PrometheusSpec) GetDir() []string {
	return []string{
		s.DeployDir,
		s.DataDir,
	}
}

// GetStatus queries current status of the instance
func (s PrometheusSpec) GetStatus(pdList ...string) string {
	return "-"
}

// Role returns the component role of the instance
func (s PrometheusSpec) Role() string {
	return RoleMonitor
}

// GrafanaSpec represents the Grafana topology specification in topology.yaml
type GrafanaSpec struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port" default:"3000"`
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
}

// GetID returns the UUID of the instance
func (s GrafanaSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s GrafanaSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s GrafanaSpec) GetPort() []int {
	return []int{
		s.Port,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s GrafanaSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s GrafanaSpec) GetDir() []string {
	return []string{
		s.DeployDir,
	}
}

// GetStatus queries current status of the instance
func (s GrafanaSpec) GetStatus(pdList ...string) string {
	return "-"
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
	UUID        string `yaml:"uuid,omitempty"`
	SSHPort     int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir   string `yaml:"deploy_dir,omitempty"`
	DataDir     string `yaml:"data_dir,omitempty"`
}

// GetID returns the UUID of the instance
func (s AlertManagerSpec) GetID() string {
	return s.UUID
}

// GetHost returns the hostname of the instance
func (s AlertManagerSpec) GetHost() string {
	return s.Host
}

// GetPort returns the port(s) of instance
func (s AlertManagerSpec) GetPort() []int {
	return []int{
		s.WebPort,
		s.ClusterPort,
	}
}

// GetSSHPort returns the SSH port of the instance
func (s AlertManagerSpec) GetSSHPort() int {
	return s.SSHPort
}

// GetDir returns the directory(ies) of the instance
func (s AlertManagerSpec) GetDir() []string {
	return []string{
		s.DeployDir,
		s.DataDir,
	}
}

// GetStatus queries current status of the instance
func (s AlertManagerSpec) GetStatus(pdList ...string) string {
	return "-"
}

// Role returns the component role of the instance
func (s AlertManagerSpec) Role() string {
	return RoleMonitor
}

/*
// TopologyGlobalOptions represents the global options for all groups in topology
// pecification in topology.yaml
type TopologyGlobalOptions struct {
	SSHPort              int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir            string `yaml:"deploy_dir,omitempty"`
	DataDir              string `yaml:"data_dir,omitempty"`
	NodeExporterPort     int    `yaml:"node_exporter_port,omitempty" default:"9100"`
	BlackboxExporterPort int    `yaml:"blackbox_exporter_port,omitempty" default:"9115"`
}
*/

// TopologySpecification represents the specification of topology.yaml
type TopologySpecification struct {
	//GlobalOptions TopologyGlobalOptions `yaml:"global,omitempty"`
	TiDBServers  []TiDBSpec         `yaml:"tidb_servers"`
	TiKVServers  []TiKVSpec         `yaml:"tikv_servers"`
	PDServers    []PDSpec           `yaml:"pd_servers"`
	PumpServers  []PumpSpec         `yaml:"pump_servers,omitempty"`
	Drainers     []DrainerSpec      `yaml:"drainer_servers,omitempty"`
	MonitorSpec  []PrometheusSpec   `yaml:"monitoring_servers"`
	Grafana      []GrafanaSpec      `yaml:"grafana_servers,omitempty"`
	Alertmanager []AlertManagerSpec `yaml:"alertmanager_servers,omitempty"`
}

// UnmarshalYAML sets default values when unmarshaling the topology file
func (topo *TopologySpecification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type topology TopologySpecification
	if err := unmarshal((*topology)(topo)); err != nil {
		return err
	}

	defaults.Set(topo)
	if err := fillCustomDefaults(topo); err != nil {
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

// fillDefaults tries to fill custom fields to their default values
func fillCustomDefaults(data interface{}) error {
	v := reflect.ValueOf(data).Elem()
	t := v.Type()

	var err error
	for i := 0; i < t.NumField(); i++ {
		if err = setCustomDefaults(v.Field(i)); err != nil {
			return err
		}
	}

	return nil
}

func setCustomDefaults(field reflect.Value) error {
	if !field.CanSet() {
		return nil
	}

	switch field.Kind() {
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			if err := setCustomDefaults(field.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		ref := reflect.New(field.Type())
		ref.Elem().Set(field)
		if err := fillCustomDefaults(ref.Interface()); err != nil {
			return err
		}
		field.Set(ref.Elem())
	case reflect.Ptr:
		if err := setCustomDefaults(field.Elem()); err != nil {
			return err
		}
	}

	if field.Kind() != reflect.Struct {
		return nil
	}

	for j := 0; j < field.NumField(); j++ {
		switch field.Type().Field(j).Name {
		case "Host":
			if field.Field(j).String() == "" {
				// TODO: remove empty server from topology
			}
		case "UUID":
			if field.Field(j).String() == "" {
				field.Field(j).Set(reflect.ValueOf(getNodeID(field)))
			}
		case "DeployDir", "DataDir":
			if field.Field(j).String() != "" {
				continue
			}

			// fill default path for empty value, default paths are relative
			// ones, when using the value, remember to check and fill base
			// paths of them.
			if defaults.CanUpdate(field.Field(j).Interface()) {
				dir := fmt.Sprintf("%s-%s",
					field.Interface().(InstanceSpec).Role(),
					getPort(field))
				field.Field(j).Set(reflect.ValueOf(dir))
			}
		}
	}

	return nil
}

// getNodeID tries to build an UUID from the node's Host and service port
func getPort(v reflect.Value) string {
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "Port", "ClientPort", "WebPort":
			return fmt.Sprintf("%d", v.Field(i).Int())
		}
	}
	return ""
}

// getNodeID tries to build an UUID from the node's Host and service port
func getNodeID(v reflect.Value) string {
	host := ""
	port := ""

	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "UUID": // return if UUID is already set
			if v.Field(i).String() != "" {
				return v.Field(i).String()
			}
		case "Host":
			host = v.Field(i).String()
		case "Port", "ClientPort", "WebPort":
			port = v.Field(i).String()
		}
	}
	return utils.UUID(fmt.Sprintf("%s:%s", host, port))
}
