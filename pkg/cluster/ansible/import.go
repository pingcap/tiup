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
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/relex/aini"
)

// ReadInventory reads the inventory files of a TiDB cluster deployed by TiDB-Ansible
func ReadInventory(ctx context.Context, dir, inventoryFileName string) (string, *spec.ClusterMeta, *aini.InventoryData, error) {
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)

	if inventoryFileName == "" {
		inventoryFileName = AnsibleInventoryFile
	}
	inventoryFile, err := os.Open(filepath.Join(dir, inventoryFileName))
	if err != nil {
		return "", nil, nil, err
	}
	defer inventoryFile.Close()

	logger.Infof("Found inventory file %s, parsing...", inventoryFile.Name())
	clsName, clsMeta, inventory, err := parseInventoryFile(inventoryFile)
	if err != nil {
		return "", nil, inventory, err
	}

	logger.Infof("Found cluster \"%s\" (%s), deployed with user %s.",
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
			GlobalOptions: spec.GlobalOptions{
				Arch: "amd64",
			},
			MonitoredOptions: spec.MonitoredOptions{},
			TiDBServers:      make([]*spec.TiDBSpec, 0),
			TiKVServers:      make([]*spec.TiKVSpec, 0),
			PDServers:        make([]*spec.PDSpec, 0),
			TiFlashServers:   make([]*spec.TiFlashSpec, 0),
			PumpServers:      make([]*spec.PumpSpec, 0),
			Drainers:         make([]*spec.DrainerSpec, 0),
			Monitors:         make([]*spec.PrometheusSpec, 0),
			Grafanas:         make([]*spec.GrafanaSpec, 0),
			Alertmanagers:    make([]*spec.AlertmanagerSpec, 0),
		},
	}
	clsName := ""

	// get global vars
	grp, ok := inventory.Groups["all"]
	if !ok || len(grp.Hosts) == 0 {
		return "", nil, inventory, errors.New("no available host in the inventory file")
	}
	// set global variables
	clsName = grp.Vars["cluster_name"]
	clsMeta.User = grp.Vars["ansible_user"]
	clsMeta.Topology.GlobalOptions.User = clsMeta.User
	clsMeta.Version = grp.Vars["tidb_version"]
	clsMeta.Topology.GlobalOptions.DeployDir = grp.Vars["deploy_dir"]
	// deploy_dir and data_dir of monitored need to be set, otherwise they will be
	// subdirs of deploy_dir in global options
	allSame := uniqueVar("deploy_dir", inventory.Groups["monitored_servers"].Hosts)
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
			clsMeta.Topology.ServerConfigs.TiDB = make(map[string]any)
		}
		clsMeta.Topology.ServerConfigs.TiDB["binlog.enable"] = enableBinlog
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

func uniqueVar(key string, hosts map[string]*aini.Host) []string {
	vars := set.NewStringSet()
	for _, h := range hosts {
		vars.Insert(h.Vars[key])
	}
	return vars.Slice()
}

// parse config files
func parseConfigFile(cfgfile string) (map[string]any, error) {
	srvConfigs := make(map[string]any)
	if _, err := toml.DecodeFile(cfgfile, &srvConfigs); err != nil {
		return nil, errors.Annotate(err, "decode toml file")
	}
	return spec.FlattenMap(srvConfigs), nil
}

func diffConfigs(configs []map[string]any) (global map[string]any, locals []map[string]any) {
	global = make(map[string]any)
	keySet := set.NewStringSet()

	// parse all configs from file
	for _, config := range configs {
		locals = append(locals, config)
		for k := range config {
			keySet.Insert(k)
		}
	}

	// summary global config
	for k := range keySet {
		valSet := set.NewAnySet(reflect.DeepEqual)
		for _, config := range locals {
			valSet.Insert(config[k])
		}
		if len(valSet.Slice()) > 1 {
			// this key can't be put into global
			continue
		}
		global[k] = valSet.Slice()[0]
	}

	// delete global config from local
	for _, config := range locals {
		for k := range global {
			delete(config, k)
		}
	}

	return
}

// CommentConfig add `#` to the head of each lines for imported configs
func CommentConfig(clsName string) error {
	dir := spec.ClusterPath(clsName, spec.AnsibleImportedConfigPath)
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".toml") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return errors.Annotatef(err, "read config file %s", path)
		}
		lines := strings.Split(string(content), "\n")
		for idx := range lines {
			lines[idx] = "# " + lines[idx]
		}
		if err := utils.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
			return errors.Annotatef(err, "write config file %s", path)
		}

		return nil
	})
	return errors.Annotate(err, "comment imported config")
}

// LoadConfig files to clusterMeta, include tidbservers, tikvservers, pdservers pumpservers and drainerservers
func LoadConfig(clsName string, cls *spec.ClusterMeta) error {
	// deal with tidb config
	configs := []map[string]any{}
	for _, srv := range cls.Topology.TiDBServers {
		prefixkey := spec.ComponentTiDB
		fname := spec.ClusterPath(clsName, spec.AnsibleImportedConfigPath, fmt.Sprintf("%s-%s-%d.toml", prefixkey, srv.Host, srv.Port))
		config, err := parseConfigFile(fname)
		if err != nil {
			return err
		}
		configs = append(configs, config)
	}
	global, locals := diffConfigs(configs)
	cls.Topology.ServerConfigs.TiDB = spec.MergeConfig(cls.Topology.ServerConfigs.TiDB, global)
	for i, local := range locals {
		cls.Topology.TiDBServers[i].Config = spec.MergeConfig(cls.Topology.TiDBServers[i].Config, local)
	}

	// deal with tikv config
	configs = []map[string]any{}
	for _, srv := range cls.Topology.TiKVServers {
		prefixkey := spec.ComponentTiKV
		fname := spec.ClusterPath(clsName, spec.AnsibleImportedConfigPath, fmt.Sprintf("%s-%s-%d.toml", prefixkey, srv.Host, srv.Port))
		config, err := parseConfigFile(fname)
		if err != nil {
			return err
		}
		configs = append(configs, config)
	}
	global, locals = diffConfigs(configs)
	cls.Topology.ServerConfigs.TiKV = spec.MergeConfig(cls.Topology.ServerConfigs.TiKV, global)
	for i, local := range locals {
		cls.Topology.TiKVServers[i].Config = spec.MergeConfig(cls.Topology.TiKVServers[i].Config, local)
	}

	// deal with pd config
	configs = []map[string]any{}
	for _, srv := range cls.Topology.PDServers {
		prefixkey := spec.ComponentPD
		fname := spec.ClusterPath(clsName, spec.AnsibleImportedConfigPath, fmt.Sprintf("%s-%s-%d.toml", prefixkey, srv.Host, srv.ClientPort))
		config, err := parseConfigFile(fname)
		if err != nil {
			return err
		}
		configs = append(configs, config)
	}
	global, locals = diffConfigs(configs)
	cls.Topology.ServerConfigs.PD = spec.MergeConfig(cls.Topology.ServerConfigs.PD, global)
	for i, local := range locals {
		cls.Topology.PDServers[i].Config = spec.MergeConfig(cls.Topology.PDServers[i].Config, local)
	}

	// deal with pump config
	configs = []map[string]any{}
	for _, srv := range cls.Topology.PumpServers {
		prefixkey := spec.ComponentPump
		fname := spec.ClusterPath(clsName, spec.AnsibleImportedConfigPath, fmt.Sprintf("%s-%s-%d.toml", prefixkey, srv.Host, srv.Port))
		config, err := parseConfigFile(fname)
		if err != nil {
			return err
		}
		configs = append(configs, config)
	}
	global, locals = diffConfigs(configs)
	cls.Topology.ServerConfigs.Pump = spec.MergeConfig(cls.Topology.ServerConfigs.Pump, global)
	for i, local := range locals {
		cls.Topology.PumpServers[i].Config = spec.MergeConfig(cls.Topology.PumpServers[i].Config, local)
	}

	// deal with drainer config
	configs = []map[string]any{}
	for _, srv := range cls.Topology.Drainers {
		prefixkey := spec.ComponentDrainer
		fname := spec.ClusterPath(clsName, spec.AnsibleImportedConfigPath, fmt.Sprintf("%s-%s-%d.toml", prefixkey, srv.Host, srv.Port))
		config, err := parseConfigFile(fname)
		if err != nil {
			return err
		}
		configs = append(configs, config)
	}
	global, locals = diffConfigs(configs)
	cls.Topology.ServerConfigs.Drainer = spec.MergeConfig(cls.Topology.ServerConfigs.Drainer, global)
	for i, local := range locals {
		cls.Topology.Drainers[i].Config = spec.MergeConfig(cls.Topology.Drainers[i].Config, local)
	}

	return nil
}
