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
	//"fmt"
	"reflect"

	"github.com/creasty/defaults"
)

const (
	TopologyFileName = "topology.yaml"
)

// TiDBSpec represents the TiDB topology specification in topology.yaml
type TiDBSpec struct {
	IP         string `yaml:"ip"`
	Port       int    `yaml:"port" default:"4000"`
	StatusPort int    `yaml:"status_port" default:"10080"`
	UUID       string `yaml:"uuid,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	NumaNode   bool   `yaml:"numa_node,omitempty"`
}

// TiKVSpec represents the TiKV topology specification in topology.yaml
type TiKVSpec struct {
	IP         string   `yaml:"ip"`
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

// PDSpec represents the PD topology specification in topology.yaml
type PDSpec struct {
	IP         string `yaml:"ip"`
	ClientPort int    `yaml:"client_port" default:"2379"`
	PeerPort   int    `yaml:"peer_port" default:"2380"`
	UUID       string `yaml:"uuid,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	DataDir    string `yaml:"data_dir,omitempty"`
	NumaNode   bool   `yaml:"numa_node,omitempty"`
}

// PumpSpec represents the Pump topology specification in topology.yaml
type PumpSpec struct {
	IP        string `yaml:"ip"`
	Port      int    `yaml:"port" default:"8250"`
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
	Offline   bool   `yaml:"offline,omitempty"`
	NumaNode  bool   `yaml:"numa_node,omitempty"`
}

// DrainerSpec represents the Drainer topology specification in topology.yaml
type DrainerSpec struct {
	IP        string `yaml:"ip"`
	Port      int    `yaml:"port" default:"8249"`
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
	CommitTS  string `yaml:"commit_ts,omitempty"`
	Offline   bool   `yaml:"offline,omitempty"`
	NumaNode  bool   `yaml:"numa_node,omitempty"`
}

// PrometheusSpec represents the Prometheus Server topology specification in topology.yaml
type PrometheusSpec struct {
	IP        string `yaml:"ip"`
	Port      int    `yaml:"port" default:"9090"`
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
	DataDir   string `yaml:"data_dir,omitempty"`
}

// GrafanaSpec represents the Grafana topology specification in topology.yaml
type GrafanaSpec struct {
	IP        string `yaml:"ip"`
	Port      int    `yaml:"port" default:"3000"`
	UUID      string `yaml:"uuid,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir string `yaml:"deploy_dir,omitempty"`
}

// AlertManagerSpec represents the AlertManager topology specification in topology.yaml
type AlertManagerSpec struct {
	IP          string `yaml:"ip"`
	WebPort     int    `yaml:"web_port" default:"9093"`
	ClusterPort int    `yaml:"cluster_port" default:"9094"`
	UUID        string `yaml:"uuid,omitempty"`
	SSHPort     int    `yaml:"ssh_port,omitempty" default:"22"`
	DeployDir   string `yaml:"deploy_dir,omitempty"`
	DataDir     string `yaml:"data_dir,omitempty"`
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
	TiDBServers  []TiDBSpec       `yaml:"tidb_servers"`
	TiKVServers  []TiKVSpec       `yaml:"tikv_servers"`
	PDServers    []PDSpec         `yaml:"pd_servers"`
	PumpServers  []PumpSpec       `yaml:"pump_servers,omitempty"`
	Drainers     []DrainerSpec    `yaml:"drainer_servers,omitempty"`
	MonitorSpec  PrometheusSpec   `yaml:"monitoring_server"`
	Grafana      GrafanaSpec      `yaml:"grafana_server,omitempty"`
	Alertmanager AlertManagerSpec `yaml:"alertmanager_server,omitempty"`
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
		setCustomDefaults(field.Elem())
	}

	if field.Kind() != reflect.Struct {
		return nil
	}

	for j := 0; j < field.NumField(); j++ {
		switch field.Type().Field(j).Name {
		case "IP":
			if field.Field(j).String() == "" {
				// TODO: remove empty server from topology
			}
		case "UUID":
			// TODO: generate UUID if not set
		case "DeployDir":
			// fill default path for empty value
			if defaults.CanUpdate(field.Field(j).Interface()) {
				field.Field(j).Set(reflect.ValueOf("/home/tidb/deploy"))
			}
		case "DataDir":
			if defaults.CanUpdate(field.Field(j).Interface()) {
				field.Field(j).Set(reflect.ValueOf("/home/tidb/data"))
			}
		}
	}

	return nil
}
