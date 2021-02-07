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

package manager

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/errutil"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
)

var (
	errNSDeploy            = errorx.NewNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", errutil.ErrTraitPreCheck)

	errNSRename              = errorx.NewNamespace("rename")
	errorRenameNameNotExist  = errNSRename.NewType("name_not_exist", errutil.ErrTraitPreCheck)
	errorRenameNameDuplicate = errNSRename.NewType("name_dup", errutil.ErrTraitPreCheck)
)

// Manager to deploy a cluster.
type Manager struct {
	sysName     string
	specManager *spec.SpecManager
	bindVersion spec.BindVersion
}

// NewManager create a Manager.
func NewManager(sysName string, specManager *spec.SpecManager, bindVersion spec.BindVersion) *Manager {
	return &Manager{
		sysName:     sysName,
		specManager: specManager,
		bindVersion: bindVersion,
	}
}

func (m *Manager) meta(name string) (metadata spec.Metadata, err error) {
	exist, err := m.specManager.Exist(name)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, perrs.Errorf("%s cluster `%s` not exists", m.sysName, name)
	}

	metadata = m.specManager.NewMetadata()
	err = m.specManager.Metadata(name, metadata)
	if err != nil {
		return metadata, err
	}

	return metadata, nil
}

func (m *Manager) confirmTopology(name, version string, topo spec.Topology, patchedRoles set.StringSet) error {
	log.Infof("Please confirm your topology:")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
	fmt.Printf("Cluster name:    %s\n", cyan.Sprint(name))
	fmt.Printf("Cluster version: %s\n", cyan.Sprint(version))
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		fmt.Printf("TLS encryption:  %s\n", cyan.Sprint("enabled"))
	}

	clusterTable := [][]string{
		// Header
		{"Type", "Host", "Ports", "OS/Arch", "Directories"},
	}

	topo.IterInstance(func(instance spec.Instance) {
		comp := instance.ComponentName()
		if patchedRoles.Exist(comp) {
			comp += " (patched)"
		}
		clusterTable = append(clusterTable, []string{
			comp,
			instance.GetHost(),
			utils.JoinInt(instance.UsedPorts(), "/"),
			cliutil.OsArch(instance.OS(), instance.Arch()),
			strings.Join(instance.UsedDirs(), ","),
		})
	})

	cliutil.PrintTable(clusterTable, true)

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")
	if len(patchedRoles) != 0 {
		log.Errorf("    3. The component marked as `patched` has been replaced by previous patch commanm.")
	}

	if spec, ok := topo.(*spec.Specification); ok {
		if len(spec.TiSparkMasters) > 0 || len(spec.TiSparkWorkers) > 0 {
			cyan := color.New(color.FgCyan, color.Bold)
			msg := cyan.Sprint(`There are TiSpark nodes defined in the topology, please note that you'll need to manually install Java Runtime Environment (JRE) 8 on the host, otherwise the TiSpark nodes will fail to start.
You may read the OpenJDK doc for a reference: https://openjdk.java.net/install/
			`)
			log.Warnf(msg)
		}
	}

	return cliutil.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func (m *Manager) sshTaskBuilder(name string, topo spec.Topology, user string, opts operator.Options) *task.Builder {
	return task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(name, "ssh", "id_rsa"),
			m.specManager.Path(name, "ssh", "id_rsa.pub"),
		).
		ClusterSSH(topo, user, opts.SSHTimeout, opts.SSHType, topo.BaseTopo().GlobalOptions.SSHType)
}
