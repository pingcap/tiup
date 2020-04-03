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
	"os"
	"path/filepath"

	"github.com/creasty/defaults"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/relex/aini"
)

// ImportAnsible imports a TiDB cluster deployed by TiDB-Ansible
func ImportAnsible(dir, inventoryFileName string) (string, *meta.ClusterMeta, error) {
	if inventoryFileName == "" {
		inventoryFileName = AnsibleInventoryFile
	}
	inventoryFile, err := os.Open(filepath.Join(dir, inventoryFileName))
	if err != nil {
		return "", nil, err
	}
	defer inventoryFile.Close()

	log.Infof("Found inventory file %s, parsing...", inventoryFile.Name())
	inventory, err := aini.Parse(inventoryFile)
	if err != nil {
		return "", nil, err
	}

	clsName, clsMeta, err := parseInventory(dir, inventory)
	if err != nil {
		return "", nil, err
	}

	// TODO: get values from templates of roles to overwrite defaults
	if err := defaults.Set(clsMeta); err != nil {
		return clsName, nil, err
	}
	return clsName, clsMeta, err
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
