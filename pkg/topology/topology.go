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

package topology

// TiDBSpec represents the TiDB topology specification in topology.yml
type TiDBSpec struct {
	IP         string `yml:"ip"`
	Port       int    `yml:"port"`
	StatusPort int    `yml:"status_port"`
	DeployDir  string `yml:"deploy_dir"`
}

// TiKVSpec represents the TiKV topology specification in topology.yml
type TiKVSpec struct {
	IP         string   `yml:"ip"`
	Port       int      `yml:"port"`
	StatusPort int      `yml:"status_port"`
	DeployDir  string   `yml:"deploy_dir"`
	Labels     []string `yml:"labels"`
}

// PDSpec represents the PD topology specification in topology.yml
type PDSpec struct {
	IP         string `yml:"ip"`
	ClientPort int    `yml:"client_port"`
	PeerPort   int    `yml:"peer_port"`
	DataDir    string `yml:"data_dir"`
	DeployDir  string `yml:"deploy_dir"`
}

// PumpSpec represents the Pump topology specification in topology.yml
type PumpSpec struct {
	IP        string `yml:"ip"`
	Port      int    `yml:"port"`
	DataDir   string `yml:"data_dir"`
	DeployDir string `yml:"deploy_dir"`
}

// DrainerSpec represents the Drainer topology specification in topology.yml
type DrainerSpec struct {
	IP        string `yml:"ip"`
	Port      int    `yml:"port"`
	DataDir   string `yml:"data_dir"`
	DeployDir string `yml:"deploy_dir"`
}

// MonitorSpec represents the Monitor topology specification in topology.yml
type MonitorSpec struct {
	IP              string `yml:"ip"`
	PrometheusPort  int    `yml:"prometheus_port"`
	PushGatewayPort int    `yml:"pushgateway_port"`
	DeployDir       string `yml:"deploy_dir"`
}

// GrafanaSpec represents the Grafana topology specification in topology.yml
type GrafanaSpec struct {
	IP        string `yml:"ip"`
	Port      int    `yml:"port"`
	DeployDir string `yml:"deploy_dir"`
}

// AlertManagerSpec represents the AlertManager topology specification in topology.yml
type AlertManagerSpec struct {
	IP          string `yml:"ip"`
	Port        int    `yml:"port"`
	ClusterPort int    `yml:"cluster_port"`
	DeployDir   string `yml:"deploy_dir"`
	DataDir     string `yml:"data_dir"`
}

// Specification represents the specification of topology.yml
type Specification struct {
	TiDBServers  []TiDBSpec
	TiKVServers  []TiKVSpec
	PDServers    []PDSpec
	PumpServers  []PumpSpec
	Drainers     []DrainerSpec
	MonitorSpec  []MonitorSpec
	Grafana      GrafanaSpec
	Alertmanager AlertManagerSpec
}
