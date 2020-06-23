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
	"fmt"
	"io"
	"os"
	"path/filepath"

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
		for _, host := range grp.Hosts {
			if host.Vars["process_supervision"] != "systemd" {
				return "", nil, inventory, errors.New("only support cluster deployed with systemd")
			}
			clsMeta.User = host.Vars["ansible_user"]
			clsMeta.Topology.GlobalOptions.User = clsMeta.User
			clsMeta.Version = host.Vars["tidb_version"]
			clsName = host.Vars["cluster_name"]

			// only read the first host, all global vars should be the same
			break
		}
	} else {
		return "", nil, inventory, errors.New("no available host in the inventory file")
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
