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
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/relex/aini"
)

// ReadInventory reads the inventory files of a TiDB cluster deployed by TiDB-Ansible
func ReadInventory(dir, inventoryFileName string) (string, *spec.ClusterMeta, *aini.InventoryData, error) {
	if inventoryFileName == "" {
		inventoryFileName = AnsibleInventoryFile
	}
	inventoryFile, err := os.Open(filepath.Join(dir, inventoryFileName))
	if err != nil {
		return "", nil, nil, err
	}
	defer inventoryFile.Close()

	log.Infof("Found inventory file %s, parsing...", inventoryFile.Name())
	clsName, clsMeta, inventory, err := parseInventoryFile(inventoryFile)
	if err != nil {
		return "", nil, inventory, err
	}

	log.Infof("Found cluster \"%s\" (%s), deployed with user %s.",
		clsName, clsMeta.Version, clsMeta.User)
	return clsName, clsMeta, inventory, err
}

func parseInventoryFile(invFile io.Reader) (string, *spec.ClusterMeta, *aini.InventoryData, error) {
	inventory, err := aini.Parse(invFile)
	if err != nil {
		return "", nil, inventory, err
	}

	clsMeta := &spec.ClusterMeta{
		Topology: &spec.Specification{
			GlobalOptions:    spec.GlobalOptions{},
			MonitoredOptions: spec.MonitoredOptions{},
			TiDBServers:      make([]spec.TiDBSpec, 0),
			TiKVServers:      make([]spec.TiKVSpec, 0),
			PDServers:        make([]spec.PDSpec, 0),
			TiFlashServers:   make([]spec.TiFlashSpec, 0),
			PumpServers:      make([]spec.PumpSpec, 0),
			Drainers:         make([]spec.DrainerSpec, 0),
			Monitors:         make([]spec.PrometheusSpec, 0),
			Grafana:          make([]spec.GrafanaSpec, 0),
			Alertmanager:     make([]spec.AlertManagerSpec, 0),
		},
	}
	clsName := ""

	// get global vars
	if grp, ok := inventory.Groups["all"]; ok && len(grp.Hosts) > 0 {
		// set global variables
		clsName = grp.Vars["cluster_name"]
		clsMeta.User = grp.Vars["ansible_user"]
		clsMeta.Topology.GlobalOptions.User = clsMeta.User
		clsMeta.Version = grp.Vars["tidb_version"]
		clsMeta.Topology.GlobalOptions.DeployDir = grp.Vars["deploy_dir"]
		// deploy_dir and data_dir of monitored need to be set, otherwise they will be
		// subdirs of deploy_dir in global options
		allSame := CheckVarSame("deploy_dir", inventory.Groups["monitored_servers"].Hosts)
		if len(allSame) == 1 {
			clsMeta.Topology.MonitoredOptions.DeployDir = allSame[0]
			clsMeta.Topology.MonitoredOptions.DataDir = filepath.Join(
				clsMeta.Topology.MonitoredOptions.DeployDir,
				"data",
			)
		} else {
			clsMeta.Topology.MonitoredOptions.DeployDir = clsMeta.Topology.GlobalOptions.DeployDir
			clsMeta.Topology.MonitoredOptions.DataDir = filepath.Join(
				clsMeta.Topology.MonitoredOptions.DeployDir,
				"data",
			)
		}


		if grp.Vars["process_supervision"] != "systemd" {
			return "", nil, inventory, errors.New("only support cluster deployed with systemd")
		}

		if enableBinlog, err := strconv.ParseBool(grp.Vars["enable_binlog"]); err == nil && enableBinlog {
			if clsMeta.Topology.ServerConfigs.TiDB == nil {
				clsMeta.Topology.ServerConfigs.TiDB = make(map[string]interface{})
			}
			clsMeta.Topology.ServerConfigs.TiDB["binlog.enable"] = enableBinlog
		}
	} else {
		return "", nil, inventory, errors.New("no available host in the inventory file")
	}

	// get location_labels variables
	if location_labels, ok := inventory.Groups["pd_servers"]; ok && len(location_labels.Vars["location_labels"]) > 0 {
		val := strings.ReplaceAll(strings.Trim(strings.Trim(location_labels.Vars["location_labels"],"["),"]"),"\"","")
        label_var := strings.Split(val,",")
		if clsMeta.Topology.ServerConfigs.PD == nil {
			clsMeta.Topology.ServerConfigs.PD = make(map[string]interface{})
		}
		clsMeta.Topology.ServerConfigs.PD["replication.location-labels"] = label_var
	}

	return clsName, clsMeta, inventory, err
}

// SSHKeyPath gets the path to default SSH private key, this is the key Ansible
// uses to connect deployment servers
func SSHKeyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s/.ssh/id_rsa", homeDir)
}

// CheckVarSame compare the variable in all Hosts same or not
func CheckVarSame (key string, hosts map[string]*aini.Host) ([]string) {
	var allVarSame []string
	for _,v := range hosts {
		if len(allVarSame) == 0 || v.Vars[key] != allVarSame[0]  {
			allVarSame = append(allVarSame,v.Vars[key])
		}
	}
	return allVarSame
}

// parse config files
func ParseConfigFile( cfgfile string ) (map[string]interface{} , error) {
	srvConfigs :=make(map[string]interface{})
	sectionRegex := regexp.MustCompile(`^\[([^:\]\s]+)(?::(\w+))?\]\s*(?:\#.*)?$`)
	reader, err := os.Open(cfgfile)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	bufreader := bufio.NewReader(reader)
	scanner := bufio.NewScanner(bufreader)
	prefixkey := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		matches := sectionRegex.FindAllStringSubmatch(line, -1)
		if matches != nil {
			prefixkey = matches[0][1]
			continue
		} else if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return nil,fmt.Errorf("Invalid section entry: '%s'. Please make sure that there are no spaces in the section entry, and that there are no other invalid characters", line)
		} else {
			keyval := strings.SplitN(strings.ReplaceAll(line," ",""),"=",2)

			if len(keyval) == 1 {
				return nil,fmt.Errorf("Bad key=value pair supplied: %s", line)
			}

			if strings.Trim(keyval[1], "\"") == "" ||strings.Contains(keyval[0],"host") || strings.Contains(keyval[0],"port") {
				continue
			}

			if prefixkey != "" {
				keyval[0] = prefixkey+"."+keyval[0]
			}

			if keyval[0] == "server.labels" {
				val := strings.Split(strings.ReplaceAll(strings.ReplaceAll(strings.Trim(strings.Trim(keyval[1],"{"),"}"),"\"","")," ",""),",")
				label_val := make(map[string]interface{})
				for i:=0;i<len(val);i++ {
					kv := strings.SplitN(val[i],"=",2)
					label_val[kv[0]] = kv[1]
				}
				srvConfigs[keyval[0]] = label_val
			} else if keyval[0] == "replication.location-labels" {
               val := strings.Split(strings.ReplaceAll(strings.ReplaceAll(strings.Trim(strings.Trim(keyval[1],"["),"]"),"\"","")," ",""),",")
               srvConfigs[keyval[0]] = val
			} else {
				srvConfigs[keyval[0]] = strings.ReplaceAll(keyval[1],"\"","")
			}
		}
	}
	return srvConfigs, nil
}


// Load config files to clusterMeta ,just include tidbservers, tikvservers and pdservers
func LoadConfig(clsName string ,cls *spec.ClusterMeta) error {
	prefixkey := ""
	var err error
	Configs :=  make(map[string]interface{})
	gConfigs := make(map[string]interface{})
	initflag := false
	for i := 0 ; i < len(cls.Topology.TiDBServers); i++ {
		prefixkey = "tidb"
		srv := cls.Topology.TiDBServers[i]
		fname := spec.ClusterPath(clsName,spec.AnsibleImportedConfigPath,fmt.Sprintf("%s-%s-%d.toml",prefixkey,srv.Host, srv.Port))

		Configs , err  = ParseConfigFile(fname)
		if initflag == false {
			gConfigs = Configs
			initflag = true
		}
		// make Configs containts
		for k, _ := range gConfigs {
			if Configs[k] == nil && gConfigs[k] != nil {
				Configs[k] = k
			}
		}

		for k, v := range  Configs {

			if cls.Topology.ServerConfigs.TiDB == nil {
				cls.Topology.ServerConfigs.TiDB = make(map[string]interface{})
			}

			if i == 0 && cls.Topology.ServerConfigs.TiDB[k] == nil {
				cls.Topology.ServerConfigs.TiDB[k] = v
			} else if cls.Topology.ServerConfigs.TiDB[k] == v {
				continue
			} else if cls.Topology.ServerConfigs.TiDB[k] == nil {
				if cls.Topology.TiDBServers[i].Config == nil {
					cls.Topology.TiDBServers[i].Config = make(map[string]interface{})
				}
				if Configs[k] != k {
					cls.Topology.TiDBServers[i].Config[k] = v
				}
			} else {
				for j := 0 ; j < i ; j++  {
					if cls.Topology.TiDBServers[j].Config == nil {
						cls.Topology.TiDBServers[j].Config = make(map[string]interface{})
					}
					cls.Topology.TiDBServers[j].Config[k] = cls.Topology.ServerConfigs.TiDB[k]
				}
				if Configs[k] != k {
					if cls.Topology.TiDBServers[i].Config == nil {
						cls.Topology.TiDBServers[i].Config = make(map[string]interface{})
					}
					cls.Topology.TiDBServers[i].Config[k] = v
				}
				delete(cls.Topology.ServerConfigs.TiDB,k)
			}
		}
	}

	Configs =  make(map[string]interface{})
	gConfigs = make(map[string]interface{})
	initflag = false
	for i := 0 ; i < len(cls.Topology.TiKVServers); i++ {
		prefixkey = "tikv"
		srv := cls.Topology.TiKVServers[i]
		fname := spec.ClusterPath(clsName,spec.AnsibleImportedConfigPath,fmt.Sprintf("%s-%s-%d.toml",prefixkey,srv.Host, srv.Port))

		Configs , err  = ParseConfigFile(fname)
		if initflag == false {
			gConfigs = Configs
			initflag = true
		}
		// make Configs containts
		for k, _ := range gConfigs {
			if Configs[k] == nil && gConfigs[k] != nil {
				Configs[k] = k
			}
		}

		for k, v := range  Configs {

			if cls.Topology.ServerConfigs.TiKV == nil {
				cls.Topology.ServerConfigs.TiKV = make(map[string]interface{})
			}
			if cls.Topology.TiKVServers[i].Config == nil {
				cls.Topology.TiKVServers[i].Config = make(map[string]interface{})
			}

			if  k == "server.labels" {
				cls.Topology.TiKVServers[i].Config[k] = v
			} else if i == 0 && cls.Topology.ServerConfigs.TiKV[k] == nil {
				cls.Topology.ServerConfigs.TiKV[k] = v
			} else if cls.Topology.ServerConfigs.TiKV[k] == v {
				continue
			} else if cls.Topology.ServerConfigs.TiKV[k] == nil {
				if Configs[k] != k {
					cls.Topology.TiKVServers[i].Config[k] = v
				}
			} else {
				for j := 0 ; j < i ; j++  {
					cls.Topology.TiKVServers[j].Config[k] = cls.Topology.ServerConfigs.TiKV[k]
				}
				if Configs[k] != k {
					cls.Topology.TiKVServers[i].Config[k] = v
				}
				delete(cls.Topology.ServerConfigs.TiKV,k)
			}
		}
	}

	Configs =  make(map[string]interface{})
	gConfigs = make(map[string]interface{})
	initflag = false
	for i := 0 ; i < len(cls.Topology.PDServers); i++ {
		prefixkey = "pd"
		srv := cls.Topology.PDServers[i]
		fname := spec.ClusterPath(clsName,spec.AnsibleImportedConfigPath,fmt.Sprintf("%s-%s-%d.toml",prefixkey,srv.Host, srv.ClientPort))

		Configs , err  = ParseConfigFile(fname)
		if initflag == false {
			gConfigs = Configs
			initflag = true
		}
		// make Configs containts
		for k, _ := range gConfigs {
			if Configs[k] == nil && gConfigs[k] != nil {
				Configs[k] = k
			}
		}

		for k, v := range  Configs {

			if cls.Topology.ServerConfigs.PD == nil {
				cls.Topology.ServerConfigs.PD = make(map[string]interface{})
			}
			if cls.Topology.PDServers[i].Config == nil {
				cls.Topology.PDServers[i].Config = make(map[string]interface{})
			}

			if   k == "replication.location-labels" && v != nil {
				cls.Topology.ServerConfigs.PD[k] = v
			} else if i == 0 && cls.Topology.ServerConfigs.PD[k] == nil {
				cls.Topology.ServerConfigs.PD[k] = v
			} else if cls.Topology.ServerConfigs.PD[k] == v {
				continue
			} else if cls.Topology.ServerConfigs.PD[k] == nil {
				if Configs[k] != k {
					cls.Topology.PDServers[i].Config[k] = v
				}
			} else {
				for j := 0 ; j < i ; j++  {
					cls.Topology.PDServers[j].Config[k] = cls.Topology.ServerConfigs.PD[k]
				}
				if Configs[k] != k {
					cls.Topology.PDServers[i].Config[k] = v
				}
				delete(cls.Topology.ServerConfigs.PD,k)
			}
		}
	}


	return err
}