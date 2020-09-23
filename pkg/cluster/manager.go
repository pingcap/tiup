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

package cluster

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/errutil"
	"github.com/pingcap/tiup/pkg/file"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"
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

// EnableCluster enable/disable the service in a cluster
func (m *Manager) EnableCluster(name string, options operator.Options, isEnable bool) error {
	if isEnable {
		log.Infof("Enabling cluster %s...", name)
	} else {
		log.Infof("Disabling cluster %s...", name)
	}

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	b := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(name, "ssh", "id_rsa"),
			m.specManager.Path(name, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, options.SSHTimeout, options.SSHType, topo.BaseTopo().GlobalOptions.SSHType)

	if isEnable {
		b = b.Func("EnableCluster", func(ctx *task.Context) error {
			return operator.Enable(ctx, topo, options, isEnable)
		})
	} else {
		b = b.Func("DisableCluster", func(ctx *task.Context) error {
			return operator.Enable(ctx, topo, options, isEnable)
		})
	}

	t := b.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if isEnable {
		log.Infof("Enabled cluster `%s` successfully", name)
	} else {
		log.Infof("Disabled cluster `%s` successfully", name)
	}

	return nil
}

// StartCluster start the cluster with specified name.
func (m *Manager) StartCluster(name string, options operator.Options, fn ...func(b *task.Builder, metadata spec.Metadata)) error {
	log.Infof("Starting cluster %s...", name)

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}

	b := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(name, "ssh", "id_rsa"),
			m.specManager.Path(name, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, options.SSHTimeout, options.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Func("StartCluster", func(ctx *task.Context) error {
			return operator.Start(ctx, topo, options, tlsCfg)
		})

	for _, f := range fn {
		f(b, metadata)
	}

	t := b.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Started cluster `%s` successfully", name)
	return nil
}

// StopCluster stop the cluster.
func (m *Manager) StopCluster(clusterName string, options operator.Options) error {
	metadata, err := m.meta(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}

	t := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.GetTopology(), base.User, options.SSHTimeout, options.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Func("StopCluster", func(ctx *task.Context) error {
			return operator.Stop(ctx, topo, options, tlsCfg)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Stopped cluster `%s` successfully", clusterName)
	return nil
}

// RestartCluster restart the cluster.
func (m *Manager) RestartCluster(clusterName string, options operator.Options) error {
	metadata, err := m.meta(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}

	t := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, options.SSHTimeout, options.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Func("RestartCluster", func(ctx *task.Context) error {
			return operator.Restart(ctx, topo, options, tlsCfg)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Restarted cluster `%s` successfully", clusterName)
	return nil
}

// ListCluster list the clusters.
func (m *Manager) ListCluster() error {
	names, err := m.specManager.List()
	if err != nil {
		return perrs.AddStack(err)
	}

	clusterTable := [][]string{
		// Header
		{"Name", "User", "Version", "Path", "PrivateKey"},
	}

	for _, name := range names {
		metadata, err := m.meta(name)
		if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
			return perrs.Trace(err)
		}

		base := metadata.GetBaseMeta()

		clusterTable = append(clusterTable, []string{
			name,
			base.User,
			base.Version,
			m.specManager.Path(name),
			m.specManager.Path(name, "ssh", "id_rsa"),
		})
	}

	cliutil.PrintTable(clusterTable, true)
	return nil
}

// CleanCluster clean the cluster without destroying it
func (m *Manager) CleanCluster(clusterName string, gOpt operator.Options, cleanOpt operator.Options, skipConfirm bool) error {
	metadata, err := m.meta(clusterName)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}

	if !skipConfirm {
		target := ""
		if cleanOpt.CleanupData && cleanOpt.CleanupLog {
			target = "data and log"
		} else if cleanOpt.CleanupData {
			target = "data"
		} else if cleanOpt.CleanupLog {
			target = "log"
		}
		if err := cliutil.PromptForConfirmOrAbortError(
			"This operation will clean %s %s cluster %s's %s.\nNodes will be ignored: %s\nRoles will be ignored: %s\nDo you want to continue? [y/N]:",
			m.sysName,
			color.HiYellowString(base.Version),
			color.HiYellowString(clusterName),
			target,
			cleanOpt.RetainDataNodes,
			cleanOpt.RetainDataRoles); err != nil {
			return err
		}
		log.Infof("Cleanup cluster...")
	}

	t := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, gOpt.SSHTimeout, gOpt.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Func("StopCluster", func(ctx *task.Context) error {
			return operator.Stop(ctx, topo, operator.Options{}, tlsCfg)
		}).
		Func("CleanupCluster", func(ctx *task.Context) error {
			return operator.Cleanup(ctx, topo, cleanOpt)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Cleanup cluster `%s` successfully", clusterName)
	return nil
}

// DestroyCluster destroy the cluster.
func (m *Manager) DestroyCluster(clusterName string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	metadata, err := m.meta(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}

	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			"This operation will destroy %s %s cluster %s and its data.\nDo you want to continue? [y/N]:",
			m.sysName,
			color.HiYellowString(base.Version),
			color.HiYellowString(clusterName)); err != nil {
			return err
		}
		log.Infof("Destroying cluster...")
	}

	t := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, gOpt.SSHTimeout, gOpt.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Func("StopCluster", func(ctx *task.Context) error {
			return operator.Stop(ctx, topo, operator.Options{
				Force: destroyOpt.Force,
			}, tlsCfg)
		}).
		Func("DestroyCluster", func(ctx *task.Context) error {
			return operator.Destroy(ctx, topo, destroyOpt)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if err := m.specManager.Remove(clusterName); err != nil {
		return perrs.Trace(err)
	}

	log.Infof("Destroyed cluster `%s` successfully", clusterName)
	return nil

}

// ExecOptions for exec shell commanm.
type ExecOptions struct {
	Command string
	Sudo    bool
}

// Exec shell command on host in the tidb cluster.
func (m *Manager) Exec(clusterName string, opt ExecOptions, gOpt operator.Options) error {
	metadata, err := m.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	filterRoles := set.NewStringSet(gOpt.Roles...)
	filterNodes := set.NewStringSet(gOpt.Nodes...)

	var shellTasks []task.Task
	uniqueHosts := map[string]int{} // host -> ssh-port
	topo.IterInstance(func(inst spec.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			if len(gOpt.Roles) > 0 && !filterRoles.Exist(inst.Role()) {
				return
			}

			if len(gOpt.Nodes) > 0 && !filterNodes.Exist(inst.GetHost()) {
				return
			}

			uniqueHosts[inst.GetHost()] = inst.GetSSHPort()
		}
	})

	for host := range uniqueHosts {
		shellTasks = append(shellTasks,
			task.NewBuilder().
				Shell(host, opt.Command, opt.Sudo).
				Build())
	}

	t := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, gOpt.SSHTimeout, gOpt.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Parallel(false, shellTasks...).
		Build()

	execCtx := task.NewContext()
	if err := t.Execute(execCtx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	// print outputs
	for host := range uniqueHosts {
		stdout, stderr, ok := execCtx.GetOutputs(host)
		if !ok {
			continue
		}
		log.Infof("Outputs of %s on %s:",
			color.CyanString(opt.Command),
			color.CyanString(host))
		if len(stdout) > 0 {
			log.Infof("%s:\n%s", color.GreenString("stdout"), stdout)
		}
		if len(stderr) > 0 {
			log.Infof("%s:\n%s", color.RedString("stderr"), stderr)
		}
	}

	return nil
}

// Display cluster meta and topology.
func (m *Manager) Display(clusterName string, opt operator.Options) error {
	metadata, err := m.meta(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	// display cluster meta
	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
	fmt.Printf("Cluster name:    %s\n", cyan.Sprint(clusterName))
	fmt.Printf("Cluster version: %s\n", cyan.Sprint(base.Version))
	fmt.Printf("SSH type:        %s\n", cyan.Sprint(topo.BaseTopo().GlobalOptions.SSHType))

	// display TLS info
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		fmt.Printf("TLS encryption:  %s\n", cyan.Sprint("enabled"))
		fmt.Printf("CA certificate:     %s\n", cyan.Sprint(
			m.specManager.Path(clusterName, spec.TLSCertKeyDir, spec.TLSCACert),
		))
		fmt.Printf("Client private key: %s\n", cyan.Sprint(
			m.specManager.Path(clusterName, spec.TLSCertKeyDir, spec.TLSClientKey),
		))
		fmt.Printf("Client certificate: %s\n", cyan.Sprint(
			m.specManager.Path(clusterName, spec.TLSCertKeyDir, spec.TLSClientCert),
		))
	}

	// display topology
	clusterTable := [][]string{
		// Header
		{"ID", "Role", "Host", "Ports", "OS/Arch", "Status", "Data Dir", "Deploy Dir"},
	}

	ctx := task.NewContext()
	err = ctx.SetSSHKeySet(m.specManager.Path(clusterName, "ssh", "id_rsa"),
		m.specManager.Path(clusterName, "ssh", "id_rsa.pub"))
	if err != nil {
		return perrs.AddStack(err)
	}

	err = ctx.SetClusterSSH(topo, base.User, opt.SSHTimeout, opt.SSHType, topo.BaseTopo().GlobalOptions.SSHType)
	if err != nil {
		return perrs.AddStack(err)
	}

	filterRoles := set.NewStringSet(opt.Roles...)
	filterNodes := set.NewStringSet(opt.Nodes...)
	pdList := topo.BaseTopo().MasterList
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, ins := range comp.Instances() {
			// apply role filter
			if len(filterRoles) > 0 && !filterRoles.Exist(ins.Role()) {
				continue
			}
			// apply node filter
			if len(filterNodes) > 0 && !filterNodes.Exist(ins.ID()) {
				continue
			}

			dataDir := "-"
			insDirs := ins.UsedDirs()
			deployDir := insDirs[0]
			if len(insDirs) > 1 {
				dataDir = insDirs[1]
			}

			tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
			if err != nil {
				return perrs.AddStack(err)
			}
			status := ins.Status(tlsCfg, pdList...)
			// Query the service status
			if status == "-" {
				e, found := ctx.GetExecutor(ins.GetHost())
				if found {
					active, _ := operator.GetServiceStatus(e, ins.ServiceName())
					if parts := strings.Split(strings.TrimSpace(active), " "); len(parts) > 2 {
						if parts[1] == "active" {
							status = "Up"
						} else {
							status = parts[1]
						}
					}
				}
			}
			clusterTable = append(clusterTable, []string{
				color.CyanString(ins.ID()),
				ins.Role(),
				ins.GetHost(),
				utils.JoinInt(ins.UsedPorts(), "/"),
				cliutil.OsArch(ins.OS(), ins.Arch()),
				formatInstanceStatus(status),
				dataDir,
				deployDir,
			})

		}
	}

	// Sort by role,host,ports
	sort.Slice(clusterTable[1:], func(i, j int) bool {
		lhs, rhs := clusterTable[i+1], clusterTable[j+1]
		// column: 1 => role, 2 => host, 3 => ports
		for _, col := range []int{1, 2} {
			if lhs[col] != rhs[col] {
				return lhs[col] < rhs[col]
			}
		}
		return lhs[3] < rhs[3]
	})

	cliutil.PrintTable(clusterTable, true)
	fmt.Printf("Total nodes: %d\n", len(clusterTable)-1)

	return nil
}

// EditConfig let the user edit the config.
func (m *Manager) EditConfig(clusterName string, skipConfirm bool) error {
	metadata, err := m.meta(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()

	data, err := yaml.Marshal(topo)
	if err != nil {
		return perrs.AddStack(err)
	}

	newTopo, err := m.editTopo(topo, data, skipConfirm)
	if err != nil {
		return perrs.AddStack(err)
	}

	if newTopo == nil {
		return nil
	}

	log.Infof("Apply the change...")
	metadata.SetTopology(newTopo)
	err = m.specManager.SaveMeta(clusterName, metadata)
	if err != nil {
		return perrs.Annotate(err, "failed to save meta")
	}

	log.Infof("Apply change successfully, please use `%s reload %s [-N <nodes>] [-R <roles>]` to reload config.", cliutil.OsArgs0(), clusterName)
	return nil
}

// Rename the cluster
func (m *Manager) Rename(clusterName string, opt operator.Options, newName string) error {
	if !utils.IsExist(m.specManager.Path(clusterName)) {
		return errorRenameNameNotExist.
			New("Cluster name '%s' not exist", clusterName).
			WithProperty(cliutil.SuggestionFromFormat("Please double check your cluster name"))
	}
	if utils.IsExist(m.specManager.Path(newName)) {
		return errorRenameNameDuplicate.
			New("Cluster name '%s' is duplicated", newName).
			WithProperty(cliutil.SuggestionFromFormat("Please specify another cluster name"))
	}

	_, err := m.meta(clusterName)
	if err != nil { // refuse renaming if current cluster topology is not valid
		return perrs.AddStack(err)
	}

	if err := os.Rename(m.specManager.Path(clusterName), m.specManager.Path(newName)); err != nil {
		return perrs.AddStack(err)
	}

	log.Infof("Rename cluster `%s` -> `%s` successfully", clusterName, newName)

	opt.Roles = []string{spec.ComponentGrafana, spec.ComponentPrometheus}
	return m.Reload(newName, opt, false)
}

// Reload the cluster.
func (m *Manager) Reload(clusterName string, opt operator.Options, skipRestart bool) error {
	sshTimeout := opt.SSHTimeout

	metadata, err := m.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	var refreshConfigTasks []*task.StepDisplay

	hasImported := false
	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch

	topo.IterInstance(func(inst spec.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			uniqueHosts[inst.GetHost()] = hostInfo{
				ssh:  inst.GetSSHPort(),
				os:   inst.OS(),
				arch: inst.Arch(),
			}
		}

		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(base.User, inst.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder().UserSSH(inst.GetHost(), inst.GetSSHPort(), base.User, opt.SSHTimeout, opt.SSHType, topo.BaseTopo().GlobalOptions.SSHType)
		if inst.IsImported() {
			switch compName := inst.ComponentName(); compName {
			case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertManager:
				version := m.bindVersion(compName, base.Version)
				tb.Download(compName, inst.OS(), inst.Arch(), version).
					CopyComponent(compName, inst.OS(), inst.Arch(), version, "", inst.GetHost(), deployDir)
			}
			hasImported = true
		}

		// Refresh all configuration
		t := tb.InitConfig(clusterName,
			base.Version,
			m.specManager,
			inst, base.User,
			opt.IgnoreConfigCheck,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  m.specManager.Path(clusterName, spec.TempConfigPath),
			}).
			BuildAsStep(fmt.Sprintf("  - Refresh config %s -> %s", inst.ComponentName(), inst.ID()))
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	monitorConfigTasks := refreshMonitoredConfigTask(
		m.specManager,
		clusterName,
		uniqueHosts,
		*topo.BaseTopo().GlobalOptions,
		topo.GetMonitoredOptions(),
		sshTimeout,
		opt.SSHType)

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return perrs.AddStack(err)
		}
	}

	tb := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, opt.SSHTimeout, opt.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		ParallelStep("+ Refresh instance configs", opt.Force, refreshConfigTasks...)

	if len(monitorConfigTasks) > 0 {
		tb = tb.ParallelStep("+ Refresh monitor configs", opt.Force, monitorConfigTasks...)
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}
	if !skipRestart {
		tb = tb.Func("UpgradeCluster", func(ctx *task.Context) error {
			return operator.Upgrade(ctx, topo, opt, tlsCfg)
		})
	}

	t := tb.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Reloaded cluster `%s` successfully", clusterName)

	return nil
}

// Upgrade the cluster.
func (m *Manager) Upgrade(clusterName string, clusterVersion string, opt operator.Options) error {
	metadata, err := m.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	var (
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueComps = map[string]struct{}{}
	)

	if err := versionCompare(base.Version, clusterVersion); err != nil {
		return err
	}

	hasImported := false
	for _, comp := range topo.ComponentsByUpdateOrder() {
		for _, inst := range comp.Instances() {
			version := m.bindVersion(inst.ComponentName(), clusterVersion)
			if version == "" {
				return perrs.Errorf("unsupported component: %v", inst.ComponentName())
			}
			compInfo := componentInfo{
				component: inst.ComponentName(),
				version:   version,
			}

			// Download component from repository
			key := fmt.Sprintf("%s-%s-%s-%s", compInfo.component, compInfo.version, inst.OS(), inst.Arch())
			if _, found := uniqueComps[key]; !found {
				uniqueComps[key] = struct{}{}
				t := task.NewBuilder().
					Download(inst.ComponentName(), inst.OS(), inst.Arch(), version).
					Build()
				downloadCompTasks = append(downloadCompTasks, t)
			}

			deployDir := clusterutil.Abs(base.User, inst.DeployDir())
			// data dir would be empty for components which don't need it
			dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(base.User, inst.LogDir())

			// Deploy component
			tb := task.NewBuilder()
			if inst.IsImported() {
				switch inst.ComponentName() {
				case spec.ComponentPrometheus, spec.ComponentGrafana, spec.ComponentAlertManager:
					tb.CopyComponent(
						inst.ComponentName(),
						inst.OS(),
						inst.Arch(),
						version,
						"", // use default srcPath
						inst.GetHost(),
						deployDir,
					)
				}
				hasImported = true
			}

			// backup files of the old version
			tb = tb.BackupComponent(inst.ComponentName(), base.Version, inst.GetHost(), deployDir)

			if deployerInstance, ok := inst.(DeployerInstance); ok {
				deployerInstance.Deploy(tb, "", deployDir, version, clusterName, clusterVersion)
			} else {
				// copy dependency component if needed
				switch inst.ComponentName() {
				case spec.ComponentTiSpark:
					env := environment.GlobalEnv()
					sparkVer, _, err := env.V1Repository().LatestStableVersion(spec.ComponentSpark, false)
					if err != nil {
						return err
					}
					tb = tb.DeploySpark(inst, sparkVer.String(), "" /* default srcPath */, deployDir)
				default:
					tb = tb.CopyComponent(
						inst.ComponentName(),
						inst.OS(),
						inst.Arch(),
						version,
						"", // use default srcPath
						inst.GetHost(),
						deployDir,
					)
				}
			}

			tb.InitConfig(
				clusterName,
				clusterVersion,
				m.specManager,
				inst,
				base.User,
				opt.IgnoreConfigCheck,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  m.specManager.Path(clusterName, spec.TempConfigPath),
				},
			)
			copyCompTasks = append(copyCompTasks, tb.Build())
		}
	}

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return err
		}
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}
	t := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, opt.SSHTimeout, opt.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Parallel(false, downloadCompTasks...).
		Parallel(false, copyCompTasks...).
		Func("UpgradeCluster", func(ctx *task.Context) error {
			return operator.Upgrade(ctx, topo, opt, tlsCfg)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	metadata.SetVersion(clusterVersion)

	if err := m.specManager.SaveMeta(clusterName, metadata); err != nil {
		return perrs.Trace(err)
	}

	if err := os.RemoveAll(m.specManager.Path(clusterName, "patch")); err != nil {
		return perrs.Trace(err)
	}

	log.Infof("Upgraded cluster `%s` successfully", clusterName)

	return nil
}

// Patch the cluster.
func (m *Manager) Patch(clusterName string, packagePath string, opt operator.Options, overwrite bool) error {
	metadata, err := m.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	if exist := utils.IsExist(packagePath); !exist {
		return perrs.New("specified package not exists")
	}

	insts, err := instancesToPatch(topo, opt)
	if err != nil {
		return err
	}
	if err := checkPackage(m.bindVersion, m.specManager, clusterName, insts[0].ComponentName(), insts[0].OS(), insts[0].Arch(), packagePath); err != nil {
		return err
	}

	var replacePackageTasks []task.Task
	for _, inst := range insts {
		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		tb := task.NewBuilder()
		tb.BackupComponent(inst.ComponentName(), base.Version, inst.GetHost(), deployDir).
			InstallPackage(packagePath, inst.GetHost(), deployDir)
		replacePackageTasks = append(replacePackageTasks, tb.Build())
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}
	t := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, opt.SSHTimeout, opt.SSHType, topo.BaseTopo().GlobalOptions.SSHType).
		Parallel(false, replacePackageTasks...).
		Func("UpgradeCluster", func(ctx *task.Context) error {
			return operator.Upgrade(ctx, topo, opt, tlsCfg)
		}).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if overwrite {
		if err := overwritePatch(m.specManager, clusterName, insts[0].ComponentName(), packagePath); err != nil {
			return err
		}
	}

	return nil
}

// ScaleOutOptions contains the options for scale out.
type ScaleOutOptions struct {
	User           string // username to login to the SSH server
	SkipCreateUser bool   // don't create user
	IdentityFile   string // path to the private key file
	UsePassword    bool   // use password instead of identity file for ssh connection
}

// DeployOptions contains the options for scale out.
// TODO: merge ScaleOutOptions, should check config too when scale out.
type DeployOptions struct {
	User              string // username to login to the SSH server
	SkipCreateUser    bool   // don't create the user
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// DeployerInstance is a instance can deploy to a target deploy directory.
type DeployerInstance interface {
	Deploy(b *task.Builder, srcPath string, deployDir string, version string, clusterName string, clusterVersion string)
}

// Deploy a cluster.
func (m *Manager) Deploy(
	clusterName string,
	clusterVersion string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	optTimeout int64,
	sshTimeout int64,
	sshType executor.SSHType,
) error {
	if err := clusterutil.ValidateClusterNameOrError(clusterName); err != nil {
		return err
	}

	exist, err := m.specManager.Exist(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	if exist {
		// FIXME: When change to use args, the suggestion text need to be updatem.
		return errDeployNameDuplicate.
			New("Cluster name '%s' is duplicated", clusterName).
			WithProperty(cliutil.SuggestionFromFormat("Please specify another cluster name"))
	}

	metadata := m.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := clusterutil.ParseTopologyYaml(topoFile, topo); err != nil {
		return err
	}

	base := topo.BaseTopo()
	if sshType != "" {
		base.GlobalOptions.SSHType = sshType
	}

	clusterList, err := m.specManager.GetAllClusters()
	if err != nil {
		return err
	}
	if err := spec.CheckClusterPortConflict(clusterList, clusterName, topo); err != nil {
		return err
	}
	if err := spec.CheckClusterDirConflict(clusterList, clusterName, topo); err != nil {
		return err
	}

	if !skipConfirm {
		if err := m.confirmTopology(clusterName, clusterVersion, topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	var sshConnProps *cliutil.SSHConnectionProps = &cliutil.SSHConnectionProps{}
	if sshType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = cliutil.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(m.specManager.Path(clusterName), 0755); err != nil {
		return errorx.InitializationFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", m.specManager.Path(clusterName)).
			WithProperty(cliutil.SuggestionFromString("Please check file system permissions and try again."))
	}

	var (
		envInitTasks      []*task.StepDisplay // tasks which are used to initialize environment
		downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	// Initialize environment
	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	globalOptions := base.GlobalOptions

	// generate CA and client cert for TLS enabled cluster
	var ca *crypto.CertificateAuthority
	if globalOptions.TLSEnabled {
		// generate CA
		tlsPath := m.specManager.Path(clusterName, spec.TLSCertKeyDir)
		if err := utils.CreateDir(tlsPath); err != nil {
			return err
		}
		ca, err = genAndSaveClusterCA(clusterName, tlsPath)
		if err != nil {
			return err
		}

		// generate client cert
		if err = genAndSaveClientCert(ca, clusterName, tlsPath); err != nil {
			return err
		}
	}

	var iterErr error // error when itering over instances
	iterErr = nil
	topo.IterInstance(func(inst spec.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			// check for "imported" parameter, it can not be true when scaling out
			if inst.IsImported() {
				iterErr = errors.New(
					"'imported' is set to 'true' for new instance, this is only used " +
						"for instances imported from tidb-ansible and make no sense when " +
						"deploying new instances, please delete the line or set it to 'false' for new instances")
				return // skip the host to avoid issues
			}

			uniqueHosts[inst.GetHost()] = hostInfo{
				ssh:  inst.GetSSHPort(),
				os:   inst.OS(),
				arch: inst.Arch(),
			}
			var dirs []string
			for _, dir := range []string{globalOptions.DeployDir, globalOptions.LogDir} {
				if dir == "" {
					continue
				}
				dirs = append(dirs, clusterutil.Abs(globalOptions.User, dir))
			}
			// the default, relative path of data dir is under deploy dir
			if strings.HasPrefix(globalOptions.DataDir, "/") {
				dirs = append(dirs, globalOptions.DataDir)
			}
			t := task.NewBuilder().
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					opt.User,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					sshTimeout,
					sshType,
					globalOptions.SSHType,
				).
				EnvInit(inst.GetHost(), globalOptions.User, globalOptions.Group, opt.SkipCreateUser || globalOptions.User == opt.User).
				Mkdir(globalOptions.User, inst.GetHost(), dirs...).
				BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", inst.GetHost(), inst.GetSSHPort()))
			envInitTasks = append(envInitTasks, t)
		}
	})

	if iterErr != nil {
		return iterErr
	}

	// Download missing component
	downloadCompTasks = BuildDownloadCompTasks(clusterVersion, topo, m.bindVersion)

	// Deploy components to remote
	topo.IterInstance(func(inst spec.Instance) {
		version := m.bindVersion(inst.ComponentName(), clusterVersion)
		deployDir := clusterutil.Abs(globalOptions.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(globalOptions.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(globalOptions.User, inst.LogDir())
		// Deploy component
		// prepare deployment server
		deployDirs := []string{
			deployDir, logDir,
			filepath.Join(deployDir, "bin"),
			filepath.Join(deployDir, "conf"),
			filepath.Join(deployDir, "scripts"),
		}
		if globalOptions.TLSEnabled {
			deployDirs = append(deployDirs, filepath.Join(deployDir, "tls"))
		}
		t := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), globalOptions.User, sshTimeout, sshType, globalOptions.SSHType).
			Mkdir(globalOptions.User, inst.GetHost(), deployDirs...).
			Mkdir(globalOptions.User, inst.GetHost(), dataDirs...)

		if deployerInstance, ok := inst.(DeployerInstance); ok {
			deployerInstance.Deploy(t, "", deployDir, version, clusterName, clusterVersion)
		} else {
			// copy dependency component if needed
			switch inst.ComponentName() {
			case spec.ComponentTiSpark:
				env := environment.GlobalEnv()
				var sparkVer v0manifest.Version
				if sparkVer, _, iterErr = env.V1Repository().LatestStableVersion(spec.ComponentSpark, false); iterErr != nil {
					return
				}
				t = t.DeploySpark(inst, sparkVer.String(), "" /* default srcPath */, deployDir)
			default:
				t = t.CopyComponent(
					inst.ComponentName(),
					inst.OS(),
					inst.Arch(),
					version,
					"", // use default srcPath
					inst.GetHost(),
					deployDir,
				)
			}
		}

		// generate and transfer tls cert for instance
		if globalOptions.TLSEnabled {
			t = t.TLSCert(inst, ca, meta.DirPaths{
				Deploy: deployDir,
				Cache:  m.specManager.Path(clusterName, spec.TempConfigPath),
			})
		}

		// generate configs for the component
		t = t.InitConfig(
			clusterName,
			clusterVersion,
			m.specManager,
			inst,
			globalOptions.User,
			opt.IgnoreConfigCheck,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  m.specManager.Path(clusterName, spec.TempConfigPath),
			},
		)

		deployCompTasks = append(deployCompTasks,
			t.BuildAsStep(fmt.Sprintf("  - Copy %s -> %s", inst.ComponentName(), inst.GetHost())),
		)
	})

	if iterErr != nil {
		return iterErr
	}

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(
		m.bindVersion,
		m.specManager,
		clusterName,
		uniqueHosts,
		globalOptions,
		topo.GetMonitoredOptions(),
		clusterVersion,
		sshTimeout,
		sshType,
	)
	downloadCompTasks = append(downloadCompTasks, dlTasks...)
	deployCompTasks = append(deployCompTasks, dpTasks...)

	builder := task.NewBuilder().
		Step("+ Generate SSH keys",
			task.NewBuilder().SSHKeyGen(m.specManager.Path(clusterName, "ssh", "id_rsa")).Build()).
		ParallelStep("+ Download TiDB components", false, downloadCompTasks...).
		ParallelStep("+ Initialize target host environments", false, envInitTasks...).
		ParallelStep("+ Copy files", false, deployCompTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, topo)
	}

	t := builder.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.AddStack(err)
	}

	metadata.SetUser(globalOptions.User)
	metadata.SetVersion(clusterVersion)
	err = m.specManager.SaveMeta(clusterName, metadata)

	if err != nil {
		return perrs.AddStack(err)
	}

	hint := color.New(color.Bold).Sprintf("%s start %s", cliutil.OsArgs0(), clusterName)
	log.Infof("Deployed cluster `%s` successfully, you can start the cluster via `%s`", clusterName, hint)
	return nil
}

// ScaleIn the cluster.
func (m *Manager) ScaleIn(
	clusterName string,
	skipConfirm bool,
	optTimeout int64,
	sshTimeout int64,
	sshType executor.SSHType,
	force bool,
	nodes []string,
	scale func(builer *task.Builder, metadata spec.Metadata, tlsCfg *tls.Config),
) error {
	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			"This operation will delete the %s nodes in `%s` and all their data.\nDo you want to continue? [y/N]:",
			strings.Join(nodes, ","),
			color.HiYellowString(clusterName)); err != nil {
			return err
		}

		if force {
			if err := cliutil.PromptForConfirmOrAbortError(
				"Forcing scale in is unsafe and may result in data lost for stateful components.\nDo you want to continue? [y/N]:",
			); err != nil {
				return err
			}
		}

		log.Infof("Scale-in nodes...")
	}

	metadata, err := m.meta(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		// ignore conflict check error, node may be deployed by former version
		// that lack of some certain conflict checks
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// Regenerate configuration
	var regenConfigTasks []task.Task
	hasImported := false
	deletedNodes := set.NewStringSet(nodes...)
	for _, component := range topo.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			if deletedNodes.Exist(instance.ID()) {
				continue
			}
			deployDir := clusterutil.Abs(base.User, instance.DeployDir())
			// data dir would be empty for components which don't need it
			dataDirs := clusterutil.MultiDirAbs(base.User, instance.DataDir())
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(base.User, instance.LogDir())

			// Download and copy the latest component to remote if the cluster is imported from Ansible
			tb := task.NewBuilder()
			if instance.IsImported() {
				switch compName := instance.ComponentName(); compName {
				case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertManager:
					version := m.bindVersion(compName, base.Version)
					tb.Download(compName, instance.OS(), instance.Arch(), version).
						CopyComponent(
							compName,
							instance.OS(),
							instance.Arch(),
							version,
							"", // use default srcPath
							instance.GetHost(),
							deployDir,
						)
				}
				hasImported = true
			}

			t := tb.InitConfig(clusterName,
				base.Version,
				m.specManager,
				instance,
				base.User,
				true, // always ignore config check result in scale in
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  m.specManager.Path(clusterName, spec.TempConfigPath),
				},
			).Build()
			regenConfigTasks = append(regenConfigTasks, t)
		}
	}

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return err
		}
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return perrs.AddStack(err)
	}

	b := task.NewBuilder().
		SSHKeySet(
			m.specManager.Path(clusterName, "ssh", "id_rsa"),
			m.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, sshTimeout, sshType, metadata.GetTopology().BaseTopo().GlobalOptions.SSHType)

	// TODO: support command scale in operation.
	scale(b, metadata, tlsCfg)

	t := b.Parallel(force, regenConfigTasks...).Parallel(force, buildDynReloadProm(metadata.GetTopology())...).Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Scaled cluster `%s` in successfully", clusterName)

	return nil
}

// ScaleOut scale out the cluster.
func (m *Manager) ScaleOut(
	clusterName string,
	topoFile string,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	final func(b *task.Builder, name string, meta spec.Metadata),
	opt ScaleOutOptions,
	skipConfirm bool,
	optTimeout int64,
	sshTimeout int64,
	sshType executor.SSHType,
) error {
	metadata, err := m.meta(clusterName)
	if err != nil { // not allowing validation errors
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// not allowing validation errors
	if err := topo.Validate(); err != nil {
		return err
	}

	// Inherit existing global configuration. We must assign the inherited values before unmarshalling
	// because some default value rely on the global options and monitored options.
	newPart := topo.NewPart()

	// The no tispark master error is ignored, as if the tispark master is removed from the topology
	// file for some reason (manual edit, for example), it is still possible to scale-out it to make
	// the whole topology back to normal state.
	if err := clusterutil.ParseTopologyYaml(topoFile, newPart); err != nil &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	if err := validateNewTopo(newPart); err != nil {
		return err
	}

	// Abort scale out operation if the merged topology is invalid
	mergedTopo := topo.MergeTopo(newPart)
	if err := mergedTopo.Validate(); err != nil {
		return err
	}

	clusterList, err := m.specManager.GetAllClusters()
	if err != nil {
		return err
	}
	if err := spec.CheckClusterPortConflict(clusterList, clusterName, mergedTopo); err != nil {
		return err
	}
	if err := spec.CheckClusterDirConflict(clusterList, clusterName, mergedTopo); err != nil {
		return err
	}

	patchedComponents := set.NewStringSet()
	newPart.IterInstance(func(instance spec.Instance) {
		if utils.IsExist(m.specManager.Path(clusterName, spec.PatchDirName, instance.ComponentName()+".tar.gz")) {
			patchedComponents.Insert(instance.ComponentName())
		}
	})

	if !skipConfirm {
		// patchedComponents are components that have been patched and overwrited
		if err := m.confirmTopology(clusterName, base.Version, newPart, patchedComponents); err != nil {
			return err
		}
	}

	var sshConnProps *cliutil.SSHConnectionProps = &cliutil.SSHConnectionProps{}
	if sshType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = cliutil.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
	}

	// Build the scale out tasks
	t, err := buildScaleOutTask(
		m, clusterName, metadata, mergedTopo, opt, sshConnProps, newPart,
		patchedComponents, optTimeout, sshTimeout, sshType, afterDeploy, final)
	if err != nil {
		return err
	}

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Scaled cluster `%s` out successfully", clusterName)

	return nil
}

func (m *Manager) meta(name string) (metadata spec.Metadata, err error) {
	exist, err := m.specManager.Exist(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	if !exist {
		return nil, perrs.Errorf("%s cluster `%s` not exists", m.sysName, name)
	}

	metadata = m.specManager.NewMetadata()
	err = m.specManager.Metadata(name, metadata)
	if err != nil {
		return metadata, perrs.AddStack(err)
	}

	return metadata, nil
}

// 1. Write Topology to a temporary file.
// 2. Open file in editor.
// 3. Check and update Topology.
// 4. Save meta file.
func (m *Manager) editTopo(origTopo spec.Topology, data []byte, skipConfirm bool) (spec.Topology, error) {
	file, err := ioutil.TempFile(os.TempDir(), "*")
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	name := file.Name()

	_, err = io.Copy(file, bytes.NewReader(data))
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	err = file.Close()
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	err = utils.OpenFileInEditor(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	// Now user finish editing the file.
	newData, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	newTopo := m.specManager.NewMetadata().GetTopology()
	err = yaml.UnmarshalStrict(newData, newTopo)
	if err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Infof("Failed to parse topology file: %v", err)
		if !cliutil.PromptForConfirmNo("Do you want to continue editing? [Y/n]: ") {
			return m.editTopo(origTopo, newData, skipConfirm)
		}
		log.Infof("Nothing changed.")
		return nil, nil
	}

	// report error if immutable field has been changed
	if err := utils.ValidateSpecDiff(origTopo, newTopo); err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Errorf("%s", err)
		if !cliutil.PromptForConfirmNo("Do you want to continue editing? [Y/n]: ") {
			return m.editTopo(origTopo, newData, skipConfirm)
		}
		log.Infof("Nothing changed.")
		return nil, nil

	}

	origData, err := yaml.Marshal(origTopo)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	if bytes.Equal(origData, newData) {
		log.Infof("The file has nothing changed")
		return nil, nil
	}

	utils.ShowDiff(string(origData), string(newData), os.Stdout)

	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			color.HiYellowString("Please check change highlight above, do you want to apply the change? [y/N]:"),
		); err != nil {
			return nil, err
		}
	}

	return newTopo, nil
}

func formatInstanceStatus(status string) string {
	lowercaseStatus := strings.ToLower(status)

	startsWith := func(prefixs ...string) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(lowercaseStatus, prefix) {
				return true
			}
		}
		return false
	}

	switch {
	case startsWith("up|l"): // up|l, up|l|ui
		return color.HiGreenString(status)
	case startsWith("up"):
		return color.GreenString(status)
	case startsWith("down", "err"): // down, down|ui
		return color.RedString(status)
	case startsWith("tombstone", "disconnected"), strings.Contains(status, "offline"):
		return color.YellowString(status)
	default:
		return status
	}
}

func versionCompare(curVersion, newVersion string) error {
	// Can always upgrade to 'nightly' event the current version is 'nightly'
	if newVersion == version.NightlyVersion {
		return nil
	}

	switch semver.Compare(curVersion, newVersion) {
	case -1:
		return nil
	case 0, 1:
		return perrs.Errorf("please specify a higher version than %s", curVersion)
	default:
		return perrs.Errorf("unreachable")
	}
}

type componentInfo struct {
	component string
	version   string
}

func instancesToPatch(topo spec.Topology, options operator.Options) ([]spec.Instance, error) {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := topo.ComponentsByStartOrder()
	components = operator.FilterComponent(components, roleFilter)

	instances := []spec.Instance{}
	comps := []string{}
	for _, com := range components {
		insts := operator.FilterInstance(com.Instances(), nodeFilter)
		if len(insts) > 0 {
			comps = append(comps, com.Name())
		}
		instances = append(instances, insts...)
	}
	if len(comps) > 1 {
		return nil, fmt.Errorf("can't patch more than one component at once: %v", comps)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instance found on specifid role(%v) and nodes(%v)", options.Roles, options.Nodes)
	}

	return instances, nil
}

func checkPackage(bindVersion spec.BindVersion, specManager *spec.SpecManager, clusterName, comp, nodeOS, arch, packagePath string) error {
	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	ver := bindVersion(comp, metadata.Version)
	repo, err := clusterutil.NewRepository(nodeOS, arch)
	if err != nil {
		return err
	}
	entry, err := repo.ComponentBinEntry(comp, ver)
	if err != nil {
		return err
	}

	checksum, err := utils.Checksum(packagePath)
	if err != nil {
		return err
	}
	cacheDir := specManager.Path(clusterName, "cache", comp+"-"+checksum[:7])
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	if err := exec.Command("tar", "-xvf", packagePath, "-C", cacheDir).Run(); err != nil {
		return err
	}

	if exists := utils.IsExist(path.Join(cacheDir, entry)); !exists {
		return fmt.Errorf("entry %s not found in package %s", entry, packagePath)
	}

	return nil
}

func overwritePatch(specManager *spec.SpecManager, clusterName, comp, packagePath string) error {
	if err := os.MkdirAll(specManager.Path(clusterName, spec.PatchDirName), 0755); err != nil {
		return err
	}

	checksum, err := utils.Checksum(packagePath)
	if err != nil {
		return err
	}

	tg := specManager.Path(clusterName, spec.PatchDirName, comp+"-"+checksum[:7]+".tar.gz")
	if !utils.IsExist(tg) {
		if err := utils.CopyFile(packagePath, tg); err != nil {
			return err
		}
	}

	symlink := specManager.Path(clusterName, spec.PatchDirName, comp+".tar.gz")
	if utils.IsSymExist(symlink) {
		os.Remove(symlink)
	}
	return os.Symlink(tg, symlink)
}

// validateNewTopo checks the new part of scale-out topology to make sure it's supported
func validateNewTopo(topo spec.Topology) (err error) {
	topo.IterInstance(func(instance spec.Instance) {
		// check for "imported" parameter, it can not be true when scaling out
		if instance.IsImported() {
			err = errors.New(
				"'imported' is set to 'true' for new instance, this is only used " +
					"for instances imported from tidb-ansible and make no sense when " +
					"scaling out, please delete the line or set it to 'false' for new instances")
			return
		}
	})
	return err
}

func (m *Manager) confirmTopology(clusterName, version string, topo spec.Topology, patchedRoles set.StringSet) error {
	log.Infof("Please confirm your topology:")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Cluster type:    %s\n", cyan.Sprint(m.sysName))
	fmt.Printf("Cluster name:    %s\n", cyan.Sprint(clusterName))
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
			comp = comp + " (patched)"
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
			log.Warnf("There are TiSpark nodes defined in the topology, please note that you'll need to manually install Java Runtime Environment (JRE) 8 on the host, other wise the TiSpark nodes will fail to start.")
			log.Warnf("You may read the OpenJDK doc for a reference: https://openjdk.java.net/install/")
		}
	}

	return cliutil.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

// Dynamic reload Prometheus configuration
func buildDynReloadProm(topo spec.Topology) []task.Task {
	monitor := spec.FindComponent(topo, spec.ComponentPrometheus)
	if monitor == nil {
		return nil
	}
	instances := monitor.Instances()
	if len(instances) == 0 {
		return nil
	}
	var dynReloadTasks []task.Task
	for _, inst := range monitor.Instances() {
		dynReloadTasks = append(dynReloadTasks, task.NewBuilder().SystemCtl(inst.GetHost(), inst.ServiceName(), "reload", true).Build())
	}
	return dynReloadTasks
}

func buildScaleOutTask(
	m *Manager,
	clusterName string,
	metadata spec.Metadata,
	mergedTopo spec.Topology,
	opt ScaleOutOptions,
	sshConnProps *cliutil.SSHConnectionProps,
	newPart spec.Topology,
	patchedComponents set.StringSet,
	optTimeout int64,
	sshTimeout int64,
	sshType executor.SSHType,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	final func(b *task.Builder, name string, meta spec.Metadata),
) (task.Task, error) {
	var (
		envInitTasks       []task.Task // tasks which are used to initialize environment
		downloadCompTasks  []task.Task // tasks which are used to download components
		deployCompTasks    []task.Task // tasks which are used to copy components to remote host
		refreshConfigTasks []task.Task // tasks which are used to refresh configuration
	)

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	specManager := m.specManager

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	// Initialize the environments
	initializedHosts := set.NewStringSet()
	metadata.GetTopology().IterInstance(func(instance spec.Instance) {
		initializedHosts.Insert(instance.GetHost())
	})
	// uninitializedHosts are hosts which haven't been initialized yet
	uninitializedHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	newPart.IterInstance(func(instance spec.Instance) {
		if host := instance.GetHost(); !initializedHosts.Exist(host) {
			if _, found := uninitializedHosts[host]; found {
				return
			}

			uninitializedHosts[host] = hostInfo{
				ssh:  instance.GetSSHPort(),
				os:   instance.OS(),
				arch: instance.Arch(),
			}

			var dirs []string
			globalOptions := metadata.GetTopology().BaseTopo().GlobalOptions
			for _, dir := range []string{globalOptions.DeployDir, globalOptions.DataDir, globalOptions.LogDir} {
				for _, dirname := range strings.Split(dir, ",") {
					if dirname == "" {
						continue
					}
					dirs = append(dirs, clusterutil.Abs(globalOptions.User, dirname))
				}
			}
			t := task.NewBuilder().
				RootSSH(
					instance.GetHost(),
					instance.GetSSHPort(),
					opt.User,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					sshTimeout,
					sshType,
					globalOptions.SSHType,
				).
				EnvInit(instance.GetHost(), base.User, base.Group, opt.SkipCreateUser || globalOptions.User == opt.User).
				Mkdir(globalOptions.User, instance.GetHost(), dirs...).
				Build()
			envInitTasks = append(envInitTasks, t)
		}
	})

	// Download missing component
	downloadCompTasks = convertStepDisplaysToTasks(BuildDownloadCompTasks(base.Version, newPart, m.bindVersion))

	var iterErr error
	// Deploy the new topology and refresh the configuration
	newPart.IterInstance(func(inst spec.Instance) {
		version := m.bindVersion(inst.ComponentName(), base.Version)
		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(base.User, inst.LogDir())

		deployDirs := []string{
			deployDir, logDir,
			filepath.Join(deployDir, "bin"),
			filepath.Join(deployDir, "conf"),
			filepath.Join(deployDir, "scripts"),
		}
		if topo.BaseTopo().GlobalOptions.TLSEnabled {
			deployDirs = append(deployDirs, filepath.Join(deployDir, "tls"))
		}
		// Deploy component
		tb := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), base.User, sshTimeout, sshType, topo.BaseTopo().GlobalOptions.SSHType).
			Mkdir(base.User, inst.GetHost(), deployDirs...).
			Mkdir(base.User, inst.GetHost(), dataDirs...)

		srcPath := ""
		if patchedComponents.Exist(inst.ComponentName()) {
			srcPath = specManager.Path(clusterName, spec.PatchDirName, inst.ComponentName()+".tar.gz")
		}

		if deployerInstance, ok := inst.(DeployerInstance); ok {
			deployerInstance.Deploy(tb, srcPath, deployDir, version, clusterName, version)
		} else {
			// copy dependency component if needed
			switch inst.ComponentName() {
			case spec.ComponentTiSpark:
				env := environment.GlobalEnv()
				var sparkVer v0manifest.Version
				if sparkVer, _, iterErr = env.V1Repository().LatestStableVersion(spec.ComponentSpark, false); iterErr != nil {
					return
				}
				tb = tb.DeploySpark(inst, sparkVer.String(), srcPath, deployDir)
			default:
				tb.CopyComponent(
					inst.ComponentName(),
					inst.OS(),
					inst.Arch(),
					version,
					srcPath,
					inst.GetHost(),
					deployDir,
				)
			}
		}
		// generate and transfer tls cert for instance
		if topo.BaseTopo().GlobalOptions.TLSEnabled {
			ca, err := crypto.ReadCA(
				clusterName,
				m.specManager.Path(clusterName, spec.TLSCertKeyDir, spec.TLSCACert),
				m.specManager.Path(clusterName, spec.TLSCertKeyDir, spec.TLSCAKey),
			)
			if err != nil {
				iterErr = err
				return
			}
			tb = tb.TLSCert(inst, ca, meta.DirPaths{
				Deploy: deployDir,
				Cache:  m.specManager.Path(clusterName, spec.TempConfigPath),
			})
		}

		t := tb.ScaleConfig(clusterName,
			base.Version,
			m.specManager,
			topo,
			inst,
			base.User,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
			},
		).Build()
		deployCompTasks = append(deployCompTasks, t)
	})
	if iterErr != nil {
		return nil, iterErr
	}

	hasImported := false

	mergedTopo.IterInstance(func(inst spec.Instance) {
		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(base.User, inst.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder()
		if inst.IsImported() {
			switch compName := inst.ComponentName(); compName {
			case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertManager:
				version := m.bindVersion(compName, base.Version)
				tb.Download(compName, inst.OS(), inst.Arch(), version).
					CopyComponent(compName, inst.OS(), inst.Arch(), version, "", inst.GetHost(), deployDir)
			}
			hasImported = true
		}

		// Refresh all configuration
		t := tb.InitConfig(clusterName,
			base.Version,
			m.specManager,
			inst,
			base.User,
			true, // always ignore config check result in scale out
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  specManager.Path(clusterName, spec.TempConfigPath),
			},
		).Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return task.NewBuilder().Build(), err
		}
	}

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(
		m.bindVersion,
		specManager,
		clusterName,
		uninitializedHosts,
		topo.BaseTopo().GlobalOptions,
		topo.BaseTopo().MonitoredOptions,
		base.Version,
		sshTimeout,
		sshType,
	)
	downloadCompTasks = append(downloadCompTasks, convertStepDisplaysToTasks(dlTasks)...)
	deployCompTasks = append(deployCompTasks, convertStepDisplaysToTasks(dpTasks)...)

	builder := task.NewBuilder().
		SSHKeySet(
			specManager.Path(clusterName, "ssh", "id_rsa"),
			specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		Parallel(false, downloadCompTasks...).
		Parallel(false, envInitTasks...).
		ClusterSSH(topo, base.User, sshTimeout, sshType, topo.BaseTopo().GlobalOptions.SSHType).
		Parallel(false, deployCompTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, newPart)
	}

	// TODO: find another way to make sure current cluster started
	builder.
		Func("StartCluster", func(ctx *task.Context) error {
			return operator.Start(ctx, metadata.GetTopology(), operator.Options{OptTimeout: optTimeout}, tlsCfg)
		}).
		ClusterSSH(newPart, base.User, sshTimeout, sshType, topo.BaseTopo().GlobalOptions.SSHType).
		Func("Save meta", func(_ *task.Context) error {
			metadata.SetTopology(mergedTopo)
			return m.specManager.SaveMeta(clusterName, metadata)
		}).
		Func("StartCluster", func(ctx *task.Context) error {
			return operator.Start(ctx, newPart, operator.Options{OptTimeout: optTimeout}, tlsCfg)
		}).
		Parallel(false, refreshConfigTasks...).
		Parallel(false, buildDynReloadProm(metadata.GetTopology())...)

	if final != nil {
		final(builder, clusterName, metadata)
	}

	return builder.Build(), nil
}

type hostInfo struct {
	ssh  int    // ssh port of host
	os   string // operating system
	arch string // cpu architecture
	// vendor string
}

// Deprecated
func convertStepDisplaysToTasks(t []*task.StepDisplay) []task.Task {
	tasks := make([]task.Task, 0, len(t))
	for _, sd := range t {
		tasks = append(tasks, sd)
	}
	return tasks
}

func buildMonitoredDeployTask(
	bindVersion spec.BindVersion,
	specManager *spec.SpecManager,
	clusterName string,
	uniqueHosts map[string]hostInfo, // host -> ssh-port, os, arch
	globalOptions *spec.GlobalOptions,
	monitoredOptions *spec.MonitoredOptions,
	version string,
	sshTimeout int64,
	sshType executor.SSHType,
) (downloadCompTasks []*task.StepDisplay, deployCompTasks []*task.StepDisplay) {
	if monitoredOptions == nil {
		return
	}

	uniqueCompOSArch := make(map[string]struct{}) // comp-os-arch -> {}
	// monitoring agents
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		version := bindVersion(comp, version)

		for host, info := range uniqueHosts {
			// populate unique os/arch set
			key := fmt.Sprintf("%s-%s-%s", comp, info.os, info.arch)
			if _, found := uniqueCompOSArch[key]; !found {
				uniqueCompOSArch[key] = struct{}{}
				downloadCompTasks = append(downloadCompTasks, task.NewBuilder().
					Download(comp, info.os, info.arch, version).
					BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)", comp, version, info.os, info.arch)))
			}

			deployDir := clusterutil.Abs(globalOptions.User, monitoredOptions.DeployDir)
			// data dir would be empty for components which don't need it
			dataDir := monitoredOptions.DataDir
			// the default data_dir is relative to deploy_dir
			if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join(deployDir, dataDir)
			}
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(globalOptions.User, monitoredOptions.LogDir)
			// Deploy component
			t := task.NewBuilder().
				UserSSH(host, info.ssh, globalOptions.User, sshTimeout, sshType, globalOptions.SSHType).
				Mkdir(globalOptions.User, host,
					deployDir, dataDir, logDir,
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "conf"),
					filepath.Join(deployDir, "scripts")).
				CopyComponent(
					comp,
					info.os,
					info.arch,
					version,
					"",
					host,
					deployDir,
				).
				MonitoredConfig(
					clusterName,
					comp,
					host,
					globalOptions.ResourceControl,
					monitoredOptions,
					globalOptions.User,
					meta.DirPaths{
						Deploy: deployDir,
						Data:   []string{dataDir},
						Log:    logDir,
						Cache:  specManager.Path(clusterName, spec.TempConfigPath),
					},
				).
				BuildAsStep(fmt.Sprintf("  - Copy %s -> %s", comp, host))
			deployCompTasks = append(deployCompTasks, t)
		}
	}
	return
}

func refreshMonitoredConfigTask(
	specManager *spec.SpecManager,
	clusterName string,
	uniqueHosts map[string]hostInfo, // host -> ssh-port, os, arch
	globalOptions spec.GlobalOptions,
	monitoredOptions *spec.MonitoredOptions,
	sshTimeout int64,
	sshType executor.SSHType,
) []*task.StepDisplay {
	if monitoredOptions == nil {
		return nil
	}

	tasks := []*task.StepDisplay{}
	// monitoring agents
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		for host, info := range uniqueHosts {
			deployDir := clusterutil.Abs(globalOptions.User, monitoredOptions.DeployDir)
			// data dir would be empty for components which don't need it
			dataDir := monitoredOptions.DataDir
			// the default data_dir is relative to deploy_dir
			if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join(deployDir, dataDir)
			}
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(globalOptions.User, monitoredOptions.LogDir)
			// Generate configs
			t := task.NewBuilder().
				UserSSH(host, info.ssh, globalOptions.User, sshTimeout, sshType, globalOptions.SSHType).
				MonitoredConfig(
					clusterName,
					comp,
					host,
					globalOptions.ResourceControl,
					monitoredOptions,
					globalOptions.User,
					meta.DirPaths{
						Deploy: deployDir,
						Data:   []string{dataDir},
						Log:    logDir,
						Cache:  specManager.Path(clusterName, spec.TempConfigPath),
					},
				).
				BuildAsStep(fmt.Sprintf("  - Refresh config %s -> %s", comp, host))
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func genAndSaveClusterCA(clusterName, tlsPath string) (*crypto.CertificateAuthority, error) {
	ca, err := crypto.NewCA(clusterName)
	if err != nil {
		return nil, err
	}

	// save CA private key
	if err := file.SaveFileWithBackup(filepath.Join(tlsPath, spec.TLSCAKey), ca.Key.Pem(), ""); err != nil {
		return nil, perrs.Annotatef(err, "cannot save CA private key for %s", clusterName)
	}

	// save CA certificate
	if err := file.SaveFileWithBackup(
		filepath.Join(tlsPath, spec.TLSCACert),
		pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: ca.Cert.Raw,
		}), ""); err != nil {
		return nil, perrs.Annotatef(err, "cannot save CA certificate for %s", clusterName)
	}

	return ca, nil
}

func genAndSaveClientCert(ca *crypto.CertificateAuthority, clusterName, tlsPath string) error {
	privKey, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return perrs.AddStack(err)
	}

	// save client private key
	if err := file.SaveFileWithBackup(filepath.Join(tlsPath, spec.TLSClientKey), privKey.Pem(), ""); err != nil {
		return perrs.Annotatef(err, "cannot save client private key for %s", clusterName)
	}

	csr, err := privKey.CSR(
		"tiup-cluster-client",
		fmt.Sprintf("%s-client", clusterName),
		[]string{}, []string{},
	)
	if err != nil {
		return perrs.Annotatef(err, "cannot generate CSR of client certificate for %s", clusterName)
	}
	cert, err := ca.Sign(csr)
	if err != nil {
		return perrs.Annotatef(err, "cannot sign client certificate for %s", clusterName)
	}

	// save client certificate
	if err := file.SaveFileWithBackup(
		filepath.Join(tlsPath, spec.TLSClientCert),
		pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		}), ""); err != nil {
		return perrs.Annotatef(err, "cannot save client PEM certificate for %s", clusterName)
	}

	// save pfx format certificate
	clientCert, err := x509.ParseCertificate(cert)
	if err != nil {
		return perrs.Annotatef(err, "cannot decode signed client certificate for %s", clusterName)
	}
	pfxData, err := privKey.PKCS12(clientCert, ca)
	if err != nil {
		return perrs.Annotatef(err, "cannot encode client certificate to PKCS#12 format for %s", clusterName)
	}
	if err := file.SaveFileWithBackup(
		filepath.Join(tlsPath, spec.PFXClientCert),
		pfxData,
		""); err != nil {
		return perrs.Annotatef(err, "cannot save client PKCS#12 certificate for %s", clusterName)
	}

	return nil
}
