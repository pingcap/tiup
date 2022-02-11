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
	"context"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

var (
	errNSDeploy            = errorx.NewNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", utils.ErrTraitPreCheck)

	errNSRename              = errorx.NewNamespace("rename")
	errorRenameNameNotExist  = errNSRename.NewType("name_not_exist", utils.ErrTraitPreCheck)
	errorRenameNameDuplicate = errNSRename.NewType("name_dup", utils.ErrTraitPreCheck)
)

// Manager to deploy a cluster.
type Manager struct {
	sysName     string
	specManager *spec.SpecManager
	bindVersion spec.BindVersion
	logger      *logprinter.Logger
}

// NewManager create a Manager.
func NewManager(
	sysName string,
	specManager *spec.SpecManager,
	bindVersion spec.BindVersion,
	logger *logprinter.Logger,
) *Manager {
	return &Manager{
		sysName:     sysName,
		specManager: specManager,
		bindVersion: bindVersion,
		logger:      logger,
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
	m.logger.Infof("Please confirm your topology:")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
	fmt.Printf("Cluster name:    %s\n", cyan.Sprint(name))
	fmt.Printf("Cluster version: %s\n", cyan.Sprint(version))
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		fmt.Printf("TLS encryption:  %s\n", cyan.Sprint("enabled"))
	}

	clusterTable := [][]string{
		// Header
		{"Role", "Host", "Ports", "OS/Arch", "Directories"},
	}

	topo.IterInstance(func(instance spec.Instance) {
		comp := instance.ComponentName()
		if patchedRoles.Exist(comp) || instance.IsPatched() {
			comp += " (patched)"
		}
		clusterTable = append(clusterTable, []string{
			comp,
			instance.GetHost(),
			utils.JoinInt(instance.UsedPorts(), "/"),
			tui.OsArch(instance.OS(), instance.Arch()),
			strings.Join(instance.UsedDirs(), ","),
		})
	})

	tui.PrintTable(clusterTable, true)

	m.logger.Warnf("Attention:")
	m.logger.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	m.logger.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")
	if len(patchedRoles) != 0 {
		m.logger.Errorf("    3. The component marked as `patched` has been replaced by previous patch commanm.")
	}

	if spec, ok := topo.(*spec.Specification); ok {
		if len(spec.TiSparkMasters) > 0 || len(spec.TiSparkWorkers) > 0 {
			cyan := color.New(color.FgCyan, color.Bold)
			msg := cyan.Sprint(`There are TiSpark nodes defined in the topology, please note that you'll need to manually install Java Runtime Environment (JRE) 8 on the host, otherwise the TiSpark nodes will fail to start.
You may read the OpenJDK doc for a reference: https://openjdk.java.net/install/
			`)
			m.logger.Warnf(msg)
		}
	}

	return tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func (m *Manager) sshTaskBuilder(name string, topo spec.Topology, user string, gOpt operator.Options) (*task.Builder, error) {
	var p *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	if gOpt.SSHType != executor.SSHTypeNone && len(gOpt.SSHProxyHost) != 0 {
		var err error
		if p, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
			return nil, err
		}
	}

	return task.NewBuilder(m.logger).
		SSHKeySet(
			m.specManager.Path(name, "ssh", "id_rsa"),
			m.specManager.Path(name, "ssh", "id_rsa.pub"),
		).
		ClusterSSH(
			topo,
			user,
			gOpt.SSHTimeout,
			gOpt.OptTimeout,
			gOpt.SSHProxyHost,
			gOpt.SSHProxyPort,
			gOpt.SSHProxyUser,
			p.Password,
			p.IdentityFile,
			p.IdentityFilePassphrase,
			gOpt.SSHProxyTimeout,
			gOpt.SSHType,
			topo.BaseTopo().GlobalOptions.SSHType,
		), nil
}

// fillHost full host cpu-arch and kernel-name
func (m *Manager) fillHost(s, p *tui.SSHConnectionProps, topo spec.Topology, gOpt *operator.Options, user string) error {
	if err := m.fillHostArchOrOS(s, p, topo, gOpt, user, spec.FullArchType); err != nil {
		return err
	}

	return m.fillHostArchOrOS(s, p, topo, gOpt, user, spec.FullOSType)
}

// fillHostArchOrOS full host cpu-arch or kernel-name
func (m *Manager) fillHostArchOrOS(s, p *tui.SSHConnectionProps, topo spec.Topology, gOpt *operator.Options, user string, fullType spec.FullHostType) error {
	globalSSHType := topo.BaseTopo().GlobalOptions.SSHType
	hostArchOrOS := map[string]string{}
	var detectTasks []*task.StepDisplay

	topo.IterInstance(func(inst spec.Instance) {
		if fullType == spec.FullOSType {
			if inst.OS() != "" {
				return
			}
		} else if inst.Arch() != "" {
			return
		}

		if _, ok := hostArchOrOS[inst.GetHost()]; ok {
			return
		}
		hostArchOrOS[inst.GetHost()] = ""

		tf := task.NewSimpleUerSSH(m.logger, inst.GetHost(), inst.GetSSHPort(), user, *gOpt, p, globalSSHType)
		if s.Password != "" {
			tf = task.NewBuilder(m.logger).
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					user,
					s.Password,
					s.IdentityFile,
					s.IdentityFilePassphrase,
					gOpt.SSHTimeout,
					gOpt.OptTimeout,
					gOpt.SSHProxyHost,
					gOpt.SSHProxyPort,
					gOpt.SSHProxyUser,
					p.Password,
					p.IdentityFile,
					p.IdentityFilePassphrase,
					gOpt.SSHProxyTimeout,
					gOpt.SSHType,
					globalSSHType,
				)
		}

		switch fullType {
		case spec.FullOSType:
			tf = tf.Shell(inst.GetHost(), "uname -s", "", false)
		default:
			tf = tf.Shell(inst.GetHost(), "uname -m", "", false)
		}
		detectTasks = append(detectTasks, tf.BuildAsStep(fmt.Sprintf("  - Detecting node %s %s info", inst.GetHost(), string(fullType))))
	})
	if len(detectTasks) == 0 {
		return nil
	}

	ctx := ctxt.New(
		context.Background(),
		gOpt.Concurrency,
		m.logger,
	)
	t := task.NewBuilder(m.logger).
		ParallelStep("+ Detect CPU Arch Or Kernel Name", false, detectTasks...).
		Build()

	if err := t.Execute(ctx); err != nil {
		return perrs.Annotate(err, "failed to fetch cpu-arch or kernel-name")
	}

	for host := range hostArchOrOS {
		stdout, _, ok := ctxt.GetInner(ctx).GetOutputs(host)
		if !ok {
			return fmt.Errorf("no check results found for %s", host)
		}
		hostArchOrOS[host] = strings.Trim(string(stdout), "\n")
	}
	return topo.FillHostArchOrOS(hostArchOrOS, fullType)
}
