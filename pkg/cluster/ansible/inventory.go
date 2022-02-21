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

package ansible

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/creasty/defaults"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/relex/aini"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v2"
)

var (
	// AnsibleInventoryFile is the default inventory file name
	AnsibleInventoryFile = "inventory.ini"
	// AnsibleConfigFile is the default ansible config file name
	AnsibleConfigFile = "ansible.cfg"
	groupVarsGlobal   = "group_vars/all.yml"
	groupVarsTiDB     = "group_vars/tidb_servers.yml"
	groupVarsTiKV     = "group_vars/tikv_servers.yml"
	groupVarsPD       = "group_vars/pd_servers.yml"
	groupVarsTiFlash  = "group_vars/tiflash_servers.yml"
	// groupVarsPump         = "group_vars/pump_servers.yml"
	// groupVarsDrainer      = "group_vars/drainer_servers.yml"
	groupVarsAlertManager = "group_vars/alertmanager_servers.yml"
	groupVarsGrafana      = "group_vars/grafana_servers.yml"
	// groupVarsMonitorAgent = "group_vars/monitored_servers.yml"
	groupVarsPrometheus = "group_vars/monitoring_servers.yml"
	// groupVarsLightning    = "group_vars/lightning_server.yml"
	// groupVarsImporter     = "group_vars/importer_server.yml"
)

// ParseAndImportInventory builds a basic ClusterMeta from the main Ansible inventory
func ParseAndImportInventory(ctx context.Context, dir, ansCfgFile string, clsMeta *spec.ClusterMeta, inv *aini.InventoryData, sshTimeout uint64, sshType executor.SSHType) error {
	if err := parseGroupVars(ctx, dir, ansCfgFile, clsMeta, inv); err != nil {
		return err
	}

	for i := 0; i < len(clsMeta.Topology.TiDBServers); i++ {
		s := clsMeta.Topology.TiDBServers[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.TiDBServers[i] = ins.(*spec.TiDBSpec)
	}
	for i := 0; i < len(clsMeta.Topology.TiKVServers); i++ {
		s := clsMeta.Topology.TiKVServers[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.TiKVServers[i] = ins.(*spec.TiKVSpec)
	}
	for i := 0; i < len(clsMeta.Topology.PDServers); i++ {
		s := clsMeta.Topology.PDServers[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.PDServers[i] = ins.(*spec.PDSpec)
	}
	for i := 0; i < len(clsMeta.Topology.TiFlashServers); i++ {
		s := clsMeta.Topology.TiFlashServers[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.TiFlashServers[i] = ins.(*spec.TiFlashSpec)
	}
	for i := 0; i < len(clsMeta.Topology.PumpServers); i++ {
		s := clsMeta.Topology.PumpServers[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.PumpServers[i] = ins.(*spec.PumpSpec)
	}
	for i := 0; i < len(clsMeta.Topology.Drainers); i++ {
		s := clsMeta.Topology.Drainers[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.Drainers[i] = ins.(*spec.DrainerSpec)
	}
	for i := 0; i < len(clsMeta.Topology.Monitors); i++ {
		s := clsMeta.Topology.Monitors[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.Monitors[i] = ins.(*spec.PrometheusSpec)
	}
	for i := 0; i < len(clsMeta.Topology.Alertmanagers); i++ {
		s := clsMeta.Topology.Alertmanagers[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.Alertmanagers[i] = ins.(*spec.AlertmanagerSpec)
	}
	for i := 0; i < len(clsMeta.Topology.Grafanas); i++ {
		s := clsMeta.Topology.Grafanas[i]
		ins, err := parseDirs(ctx, clsMeta.User, s, sshTimeout, sshType)
		if err != nil {
			return err
		}
		clsMeta.Topology.Grafanas[i] = ins.(*spec.GrafanaSpec)
	}

	// TODO: get values from templates of roles to overwrite defaults
	return defaults.Set(clsMeta)
}

func parseGroupVars(ctx context.Context, dir, ansCfgFile string, clsMeta *spec.ClusterMeta, inv *aini.InventoryData) error {
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)

	// set global vars in group_vars/all.yml
	grpVarsAll, err := readGroupVars(dir, groupVarsGlobal)
	if err != nil {
		return err
	}
	if port, ok := grpVarsAll["blackbox_exporter_port"]; ok {
		clsMeta.Topology.MonitoredOptions.BlackboxExporterPort, _ = strconv.Atoi(port)
	}
	if port, ok := grpVarsAll["node_exporter_port"]; ok {
		clsMeta.Topology.MonitoredOptions.NodeExporterPort, _ = strconv.Atoi(port)
	}

	// read ansible config
	ansCfg, err := readAnsibleCfg(ansCfgFile)
	if err != nil {
		return err
	}

	// try to set value in global ansible config to global options
	// NOTE: we read this value again when setting port for each host, because the
	// default host read from aini might be different from the one in ansible config
	if ansCfg != nil {
		rPort, err := ansCfg.Section("defaults").Key("remote_port").Int()
		if err == nil {
			clsMeta.Topology.GlobalOptions.SSHPort = rPort
		}
	}

	// set hosts
	// tidb_servers
	if grp, ok := inv.Groups["tidb_servers"]; ok && len(grp.Hosts) > 0 {
		grpVars, err := readGroupVars(dir, groupVarsTiDB)
		if err != nil {
			return err
		}
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.TiDBSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			if port, ok := grpVars["tidb_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			if statusPort, ok := grpVars["tidb_status_port"]; ok {
				tmpIns.StatusPort, _ = strconv.Atoi(statusPort)
			}

			// apply values from the host
			if port, ok := srv.Vars["tidb_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			if statusPort, ok := srv.Vars["tidb_status_port"]; ok {
				tmpIns.StatusPort, _ = strconv.Atoi(statusPort)
			}
			if logDir, ok := srv.Vars["tidb_log_dir"]; ok {
				tmpIns.LogDir = strings.Trim(logDir, "\"")
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.TiDBServers = append(clsMeta.Topology.TiDBServers, tmpIns)
		}
		logger.Infof("Imported %d TiDB node(s).", len(clsMeta.Topology.TiDBServers))
	}

	// tikv_servers
	if grp, ok := inv.Groups["tikv_servers"]; ok && len(grp.Hosts) > 0 {
		grpVars, err := readGroupVars(dir, groupVarsTiKV)
		if err != nil {
			return err
		}
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.TiKVSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			if port, ok := grpVars["tikv_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			if statusPort, ok := grpVars["tikv_status_port"]; ok {
				tmpIns.StatusPort, _ = strconv.Atoi(statusPort)
			}

			// apply values from the host
			if port, ok := srv.Vars["tikv_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			if statusPort, ok := srv.Vars["tikv_status_port"]; ok {
				tmpIns.StatusPort, _ = strconv.Atoi(statusPort)
			}
			if dataDir, ok := srv.Vars["tikv_data_dir"]; ok {
				tmpIns.DataDir = strings.Trim(dataDir, "\"")
			}
			if logDir, ok := srv.Vars["tikv_log_dir"]; ok {
				tmpIns.LogDir = strings.Trim(logDir, "\"")
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.TiKVServers = append(clsMeta.Topology.TiKVServers, tmpIns)
		}
		logger.Infof("Imported %d TiKV node(s).", len(clsMeta.Topology.TiKVServers))
	}

	// pd_servers
	if grp, ok := inv.Groups["pd_servers"]; ok && len(grp.Hosts) > 0 {
		grpVars, err := readGroupVars(dir, groupVarsPD)
		if err != nil {
			return err
		}
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.PDSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}
			if tmpIns.Host != srv.Name {
				tmpIns.Name = srv.Name // use alias as the name of PD
			}

			if clientPort, ok := grpVars["pd_client_port"]; ok {
				tmpIns.ClientPort, _ = strconv.Atoi(clientPort)
			}
			if peerPort, ok := grpVars["pd_peer_port"]; ok {
				tmpIns.PeerPort, _ = strconv.Atoi(peerPort)
			}

			// apply values from the host
			if clientPort, ok := srv.Vars["pd_client_port"]; ok {
				tmpIns.ClientPort, _ = strconv.Atoi(clientPort)
			}
			if peerPort, ok := srv.Vars["pd_peer_port"]; ok {
				tmpIns.PeerPort, _ = strconv.Atoi(peerPort)
			}
			if dataDir, ok := srv.Vars["pd_data_dir"]; ok {
				tmpIns.DataDir = strings.Trim(dataDir, "\"")
			}
			if logDir, ok := srv.Vars["pd_log_dir"]; ok {
				tmpIns.LogDir = strings.Trim(logDir, "\"")
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.PDServers = append(clsMeta.Topology.PDServers, tmpIns)
		}
		logger.Infof("Imported %d PD node(s).", len(clsMeta.Topology.PDServers))
	}

	// tiflash_servers
	if grp, ok := inv.Groups["tiflash_servers"]; ok && len(grp.Hosts) > 0 {
		grpVars, err := readGroupVars(dir, groupVarsTiFlash)
		if err != nil {
			return err
		}
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.TiFlashSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			if tcpPort, ok := grpVars["tcp_port"]; ok {
				tmpIns.TCPPort, _ = strconv.Atoi(tcpPort)
			}
			if httpPort, ok := grpVars["http_port"]; ok {
				tmpIns.HTTPPort, _ = strconv.Atoi(httpPort)
			}
			if flashServicePort, ok := grpVars["flash_service_port"]; ok {
				tmpIns.FlashServicePort, _ = strconv.Atoi(flashServicePort)
			}
			if flashProxyPort, ok := grpVars["flash_proxy_port"]; ok {
				tmpIns.FlashProxyPort, _ = strconv.Atoi(flashProxyPort)
			}
			if flashProxyStatusPort, ok := grpVars["flash_proxy_status_port"]; ok {
				tmpIns.FlashProxyStatusPort, _ = strconv.Atoi(flashProxyStatusPort)
			}
			if statusPort, ok := grpVars["metrics_port"]; ok {
				tmpIns.StatusPort, _ = strconv.Atoi(statusPort)
			}

			// apply values from the host
			if tcpPort, ok := srv.Vars["tcp_port"]; ok {
				tmpIns.TCPPort, _ = strconv.Atoi(tcpPort)
			}
			if httpPort, ok := srv.Vars["http_port"]; ok {
				tmpIns.HTTPPort, _ = strconv.Atoi(httpPort)
			}
			if flashServicePort, ok := srv.Vars["flash_service_port"]; ok {
				tmpIns.FlashServicePort, _ = strconv.Atoi(flashServicePort)
			}
			if flashProxyPort, ok := srv.Vars["flash_proxy_port"]; ok {
				tmpIns.FlashProxyPort, _ = strconv.Atoi(flashProxyPort)
			}
			if flashProxyStatusPort, ok := srv.Vars["flash_proxy_status_port"]; ok {
				tmpIns.FlashProxyStatusPort, _ = strconv.Atoi(flashProxyStatusPort)
			}
			if statusPort, ok := srv.Vars["metrics_port"]; ok {
				tmpIns.StatusPort, _ = strconv.Atoi(statusPort)
			}
			if dataDir, ok := srv.Vars["data_dir"]; ok {
				tmpIns.DataDir = strings.Trim(dataDir, "\"")
			}
			if logDir, ok := srv.Vars["tiflash_log_dir"]; ok {
				tmpIns.LogDir = strings.Trim(logDir, "\"")
			}
			if tmpDir, ok := srv.Vars["tmp_path"]; ok {
				tmpIns.TmpDir = tmpDir
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.TiFlashServers = append(clsMeta.Topology.TiFlashServers, tmpIns)
		}
		logger.Infof("Imported %d TiFlash node(s).", len(clsMeta.Topology.TiFlashServers))
	}

	// spark_master
	// spark_slaves
	// lightning_server
	// importer_server

	// monitoring_servers
	if grp, ok := inv.Groups["monitoring_servers"]; ok && len(grp.Hosts) > 0 {
		grpVars, err := readGroupVars(dir, groupVarsPrometheus)
		if err != nil {
			return err
		}
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.PrometheusSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			if port, ok := grpVars["prometheus_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			// pushgateway no longer needed, just ignore
			// NOTE: storage retention is not used at present, only for record
			if _retention, ok := grpVars["prometheus_storage_retention"]; ok {
				tmpIns.Retention = _retention
			}

			// apply values from the host
			if port, ok := srv.Vars["prometheus_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			// NOTE: storage retention is not used at present, only for record
			if _retention, ok := srv.Vars["prometheus_storage_retention"]; ok {
				tmpIns.Retention = _retention
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.Monitors = append(clsMeta.Topology.Monitors, tmpIns)
		}
		logger.Infof("Imported %d monitoring node(s).", len(clsMeta.Topology.Monitors))
	}

	// monitored_servers
	// ^- ignore, we use auto generated full list

	// alertmanager_servers
	if grp, ok := inv.Groups["alertmanager_servers"]; ok && len(grp.Hosts) > 0 {
		grpVars, err := readGroupVars(dir, groupVarsAlertManager)
		if err != nil {
			return err
		}
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.AlertmanagerSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			if port, ok := grpVars["alertmanager_port"]; ok {
				tmpIns.WebPort, _ = strconv.Atoi(port)
			}
			if clusterPort, ok := grpVars["alertmanager_cluster_port"]; ok {
				tmpIns.ClusterPort, _ = strconv.Atoi(clusterPort)
			}

			// apply values from the host
			if port, ok := srv.Vars["alertmanager_port"]; ok {
				tmpIns.WebPort, _ = strconv.Atoi(port)
			}
			if clusterPort, ok := srv.Vars["alertmanager_cluster_port"]; ok {
				tmpIns.ClusterPort, _ = strconv.Atoi(clusterPort)
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.Alertmanagers = append(clsMeta.Topology.Alertmanagers, tmpIns)
		}
		logger.Infof("Imported %d Alertmanager node(s).", len(clsMeta.Topology.Alertmanagers))
	}

	// grafana_servers
	if grp, ok := inv.Groups["grafana_servers"]; ok && len(grp.Hosts) > 0 {
		grpVars, err := readGroupVars(dir, groupVarsGrafana)
		if err != nil {
			return err
		}
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.GrafanaSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			if port, ok := grpVars["grafana_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}

			// apply values from the host
			if port, ok := srv.Vars["grafana_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}

			if username, ok := srv.Vars["grafana_admin_user"]; ok {
				tmpIns.Username = strings.Trim(username, "\"")
			}
			if passwd, ok := srv.Vars["grafana_admin_password"]; ok {
				tmpIns.Password = strings.Trim(passwd, "\"")
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.Grafanas = append(clsMeta.Topology.Grafanas, tmpIns)
		}
		logger.Infof("Imported %d Grafana node(s).", len(clsMeta.Topology.Grafanas))
	}

	// kafka_exporter_servers

	// pump_servers
	if grp, ok := inv.Groups["pump_servers"]; ok && len(grp.Hosts) > 0 {
		/*
			grpVars, err := readGroupVars(dir, groupVarsPump)
			if err != nil {
				return err
			}
		*/
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.PumpSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			// nothing in pump_servers.yml
			if port, ok := grpVarsAll["pump_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			// apply values from the host
			if port, ok := srv.Vars["pump_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			if dataDir, ok := srv.Vars["pump_data_dir"]; ok {
				tmpIns.DataDir = strings.Trim(dataDir, "\"")
			}
			if logDir, ok := srv.Vars["pump_log_dir"]; ok {
				tmpIns.LogDir = strings.Trim(logDir, "\"")
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.PumpServers = append(clsMeta.Topology.PumpServers, tmpIns)
		}
		logger.Infof("Imported %d Pump node(s).", len(clsMeta.Topology.PumpServers))
	}

	// drainer_servers
	if grp, ok := inv.Groups["drainer_servers"]; ok && len(grp.Hosts) > 0 {
		/*
			grpVars, err := readGroupVars(dir, groupVarsDrainer)
			if err != nil {
				return err
			}
		*/
		for _, srv := range grp.Hosts {
			host := srv.Vars["ansible_host"]
			if host == "" {
				host = srv.Name
			}
			tmpIns := &spec.DrainerSpec{
				Host:     host,
				SSHPort:  getHostPort(srv, ansCfg),
				Imported: true,
				Arch:     "amd64",
				OS:       "linux",
			}

			// nothing in drainer_servers.yml
			if port, ok := grpVarsAll["drainer_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}
			// apply values from the host
			if port, ok := srv.Vars["drainer_port"]; ok {
				tmpIns.Port, _ = strconv.Atoi(port)
			}

			logger.Debugf("Imported %s node %s:%d.", tmpIns.Role(), tmpIns.Host, tmpIns.GetMainPort())

			clsMeta.Topology.Drainers = append(clsMeta.Topology.Drainers, tmpIns)
		}
		logger.Infof("Imported %d Drainer node(s).", len(clsMeta.Topology.Drainers))
	}

	// TODO: node_exporter and blackbox_exporter on custom port is not supported yet
	// if it is set on a host line. Global values in group_vars/all.yml will be
	// correctly parsed.

	return nil
}

// readGroupVars sets values from configs in group_vars/ dir
func readGroupVars(dir, filename string) (map[string]string, error) {
	result := make(map[string]string)

	fileData, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(fileData, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetHostPort tries to read the SSH port of the host
func GetHostPort(host *aini.Host, cfg *ini.File) int {
	return getHostPort(host, cfg)
}

// getHostPort tries to read the SSH port of the host
// 1. get from Host.Vars["ansible_port"]
// 2. get from cfg.Section("defaults").Key("remote_port")
// 3. get from srv.Port
func getHostPort(srv *aini.Host, cfg *ini.File) int {
	// parse per host config
	// aini parse the port inline with hostnames (e.g., something like `host:22`)
	// but not handling the "ansible_port" variable
	if port, ok := srv.Vars["ansible_port"]; ok {
		intPort, err := strconv.Atoi(port)
		if err == nil {
			return intPort
		}
	}

	// try to get value from global ansible config
	if cfg != nil {
		rPort, err := cfg.Section("defaults").Key("remote_port").Int()
		if err == nil {
			return rPort
		}
	}

	return srv.Port
}

// readAnsibleCfg tries to read global configs of ansible
func readAnsibleCfg(cfgFile string) (*ini.File, error) {
	var cfgData []byte // raw config ini data
	var err error
	// try to read from env if the file does not exist
	if _, err = os.Stat(cfgFile); cfgFile == "" || err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		data := os.Getenv("ANSIBLE_CONFIG")
		if data == "" {
			return nil, nil
		}
		cfgData = []byte(data)
	} else {
		cfgData, err = os.ReadFile(cfgFile)
		if err != nil {
			return nil, err
		}
	}

	return ini.Load(cfgData)
}
