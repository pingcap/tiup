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

package operator

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
)

// Destroy the cluster.
func Destroy(
	ctx context.Context,
	cluster spec.Topology,
	options Options,
) error {
	coms := cluster.ComponentsByStopOrder()

	instCount := map[string]int{}
	cluster.IterInstance(func(inst spec.Instance) {
		instCount[inst.GetManageHost()]++
	})

	for _, com := range coms {
		insts := com.Instances()
		err := DestroyComponent(ctx, insts, cluster, options)
		if err != nil && !options.Force {
			return perrs.Annotatef(err, "failed to destroy %s", com.Name())
		}
		for _, inst := range insts {
			instCount[inst.GetManageHost()]--
			if instCount[inst.GetManageHost()] == 0 {
				if cluster.GetMonitoredOptions() != nil {
					if err := DestroyMonitored(ctx, inst, cluster.GetMonitoredOptions(), options.OptTimeout, cluster.BaseTopo().GlobalOptions.SystemdMode); err != nil && !options.Force {
						return err
					}
				}
			}
		}
	}

	gOpts := cluster.BaseTopo().GlobalOptions

	// Delete all global deploy directory
	for host := range instCount {
		if err := DeleteGlobalDirs(ctx, host, gOpts); err != nil {
			return nil
		}
	}

	// after all things done, try to remove SSH public key
	for host := range instCount {
		if err := DeletePublicKey(ctx, host); err != nil {
			return nil
		}
	}

	return nil
}

// StopAndDestroyInstance stop and destroy the instance,
// if this instance is the host's last one, and the host has monitor deployed,
// we need to destroy the monitor, too
func StopAndDestroyInstance(
	ctx context.Context,
	cluster spec.Topology,
	instance spec.Instance,
	options Options,
	forceStop bool,
	destroyNode bool,
	tlsCfg *tls.Config,
) error {
	ignoreErr := options.Force
	compName := instance.ComponentName()
	noAgentHosts := set.NewStringSet()
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)

	cluster.IterInstance(func(inst spec.Instance) {
		if inst.IgnoreMonitorAgent() {
			noAgentHosts.Insert(inst.GetManageHost())
		}
	})

	// just try to stop and destroy
	if err := StopComponent(
		ctx,
		cluster,
		[]spec.Instance{instance},
		noAgentHosts,
		options,
		forceStop, /* forceStop */
		false,     /* evictLeader */
		tlsCfg,    /* when the forceStop is false, this is use for TiCDC graceful shutdown */
	); err != nil {
		if !ignoreErr {
			return perrs.Annotatef(err, "failed to stop %s", compName)
		}
		logger.Warnf("failed to stop %s: %v", compName, err)
	}
	if err := DestroyComponent(ctx, []spec.Instance{instance}, cluster, options); err != nil {
		if !ignoreErr {
			return perrs.Annotatef(err, "failed to destroy %s", compName)
		}
		logger.Warnf("failed to destroy %s: %v", compName, err)
	}

	if destroyNode {
		// monitoredOptions for dm cluster is nil
		monitoredOptions := cluster.GetMonitoredOptions()

		if monitoredOptions != nil && !instance.IgnoreMonitorAgent() {
			if err := StopMonitored(ctx, []string{instance.GetManageHost()}, noAgentHosts, monitoredOptions, options.OptTimeout, string(cluster.BaseTopo().GlobalOptions.SystemdMode)); err != nil {
				if !ignoreErr {
					return perrs.Annotatef(err, "failed to stop monitor")
				}
				logger.Warnf("failed to stop %s: %v", "monitor", err)
			}
			if err := DestroyMonitored(ctx, instance, monitoredOptions, options.OptTimeout, cluster.BaseTopo().GlobalOptions.SystemdMode); err != nil {
				if !ignoreErr {
					return perrs.Annotatef(err, "failed to destroy monitor")
				}
				logger.Warnf("failed to destroy %s: %v", "monitor", err)
			}
		}

		if err := DeletePublicKey(ctx, instance.GetManageHost()); err != nil {
			if !ignoreErr {
				return perrs.Annotatef(err, "failed to delete public key")
			}
			logger.Warnf("failed to delete public key")
		}
	}
	return nil
}

// DeleteGlobalDirs deletes all global directories if they are empty
func DeleteGlobalDirs(ctx context.Context, host string, options *spec.GlobalOptions) error {
	if options == nil {
		return nil
	}

	e := ctxt.GetInner(ctx).Get(host)
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	logger.Infof("Clean global directories %s", host)
	for _, dir := range []string{options.LogDir, options.DeployDir, options.DataDir} {
		if dir == "" {
			continue
		}
		dir = spec.Abs(options.User, dir)

		logger.Infof("\tClean directory %s on instance %s", dir, host)

		c := module.ShellModuleConfig{
			Command:  fmt.Sprintf("rmdir %s > /dev/null 2>&1 || true", dir),
			Chdir:    "",
			UseShell: false,
		}
		shell := module.NewShellModule(c)
		stdout, stderr, err := shell.Execute(ctx, e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			logger.Errorf("%s", string(stderr))
		}

		if err != nil {
			return perrs.Annotatef(err, "failed to clean directory %s on: %s", dir, host)
		}
	}

	logger.Infof("Clean global directories %s success", host)
	return nil
}

// DeletePublicKey deletes the SSH public key from host
func DeletePublicKey(ctx context.Context, host string) error {
	e := ctxt.GetInner(ctx).Get(host)
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	logger.Infof("Delete public key %s", host)
	_, pubKeyPath := ctxt.GetInner(ctx).GetSSHKeySet()
	publicKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return perrs.Trace(err)
	}

	pubKey := string(bytes.TrimSpace(publicKey))
	pubKey = strings.ReplaceAll(pubKey, "/", "\\/")
	pubKeysFile := executor.FindSSHAuthorizedKeysFile(ctx, e)

	// delete the public key with Linux `sed` toolkit
	c := module.ShellModuleConfig{
		Command:  fmt.Sprintf("sed -i '/%s/d' %s", pubKey, pubKeysFile),
		UseShell: false,
	}
	shell := module.NewShellModule(c)
	stdout, stderr, err := shell.Execute(ctx, e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 {
		logger.Errorf("%s", string(stderr))
	}

	if err != nil {
		return perrs.Annotatef(err, "failed to delete pulblic key on: %s", host)
	}

	logger.Infof("Delete public key %s success", host)
	return nil
}

// DestroyMonitored destroy the monitored service.
func DestroyMonitored(ctx context.Context, inst spec.Instance, options *spec.MonitoredOptions, timeout uint64, systemdMode spec.SystemdMode) error {
	e := ctxt.GetInner(ctx).Get(inst.GetManageHost())
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)

	logger.Infof("Destroying monitored %s", inst.GetManageHost())
	logger.Infof("\tDestroying instance %s", inst.GetManageHost())

	// Stop by systemd.
	delPaths := make([]string, 0)

	delPaths = append(delPaths, options.DataDir)
	delPaths = append(delPaths, options.LogDir)

	// In TiDB-Ansible, deploy dir are shared by all components on the same
	// host, so not deleting it.
	if !inst.IsImported() {
		delPaths = append(delPaths, options.DeployDir)
	} else {
		logger.Warnf("Monitored deploy dir %s not deleted for TiDB-Ansible imported instance %s.",
			options.DeployDir, inst.InstanceName())
	}
	systemdDir := "/etc/systemd/system/"
	sudo := true
	if systemdMode == spec.UserMode {
		systemdDir = "~/.config/systemd/user/"
		sudo = false
	}
	delPaths = append(delPaths, fmt.Sprintf("%s%s-%d.service", systemdDir, spec.ComponentNodeExporter, options.NodeExporterPort))
	delPaths = append(delPaths, fmt.Sprintf("%s%s-%d.service", systemdDir, spec.ComponentBlackboxExporter, options.BlackboxExporterPort))

	c := module.ShellModuleConfig{
		Command:  fmt.Sprintf("rm -rf %s;", strings.Join(delPaths, " ")),
		Sudo:     sudo, // the .service files are in a directory owned by root
		Chdir:    "",
		UseShell: false,
	}
	shell := module.NewShellModule(c)
	stdout, stderr, err := shell.Execute(ctx, e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 {
		logger.Errorf("%s", string(stderr))
	}

	if err != nil {
		return perrs.Annotatef(err, "failed to destroy monitored: %s", inst.GetManageHost())
	}

	if err := spec.PortStopped(ctx, e, options.NodeExporterPort, timeout); err != nil {
		str := fmt.Sprintf("%s failed to destroy node exportoer: %s", inst.GetManageHost(), err)
		logger.Errorf("%s", str)
		return perrs.Annotatef(err, "%s", str)
	}
	if err := spec.PortStopped(ctx, e, options.BlackboxExporterPort, timeout); err != nil {
		str := fmt.Sprintf("%s failed to destroy blackbox exportoer: %s", inst.GetManageHost(), err)
		logger.Errorf("%s", str)
		return perrs.Annotatef(err, "%s", str)
	}

	logger.Infof("Destroy monitored on %s success", inst.GetManageHost())

	return nil
}

// CleanupComponent cleanup the instances
func CleanupComponent(ctx context.Context, delFileMaps map[string]set.StringSet, sudo bool) error {
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	for host, delFiles := range delFileMaps {
		e := ctxt.GetInner(ctx).Get(host)
		logger.Infof("Cleanup instance %s", host)
		logger.Debugf("Deleting paths on %s: %s", host, strings.Join(delFiles.Slice(), " "))
		c := module.ShellModuleConfig{
			Command:  fmt.Sprintf("rm -rf %s;", strings.Join(delFiles.Slice(), " ")),
			Sudo:     sudo, // the .service files are in a directory owned by root
			Chdir:    "",
			UseShell: true,
		}
		shell := module.NewShellModule(c)
		stdout, stderr, err := shell.Execute(ctx, e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			logger.Errorf("%s", string(stderr))
		}

		if err != nil {
			return perrs.Annotatef(err, "failed to cleanup: %s", host)
		}

		logger.Infof("Cleanup %s success", host)
	}

	return nil
}

// DestroyComponent destroy the instances.
func DestroyComponent(ctx context.Context, instances []spec.Instance, cls spec.Topology, options Options) error {
	if len(instances) == 0 {
		return nil
	}

	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	name := instances[0].ComponentName()
	logger.Infof("Destroying component %s", name)

	retainDataRoles := set.NewStringSet(options.RetainDataRoles...)
	retainDataNodes := set.NewStringSet(options.RetainDataNodes...)

	for _, ins := range instances {
		// Some data of instances will be retained
		dataRetained := retainDataRoles.Exist(ins.ComponentName()) ||
			retainDataNodes.Exist(ins.ID()) || retainDataNodes.Exist(ins.GetHost()) || retainDataRoles.Exist(ins.GetManageHost())

		e := ctxt.GetInner(ctx).Get(ins.GetManageHost())
		logger.Infof("\tDestroying instance %s", ins.GetManageHost())

		var dataDirs []string
		if len(ins.DataDir()) > 0 {
			dataDirs = strings.Split(ins.DataDir(), ",")
		}

		deployDir := ins.DeployDir()
		delPaths := set.NewStringSet()

		// Retain the deploy directory if the users want to retain the data directory
		// and the data directory is a sub-directory of deploy directory
		keepDeployDir := false

		for _, dataDir := range dataDirs {
			// Don't delete the parent directory if any sub-directory retained
			keepDeployDir = (dataRetained && strings.HasPrefix(dataDir, deployDir)) || keepDeployDir
			if !dataRetained && cls.CountDir(ins.GetManageHost(), dataDir) == 1 {
				// only delete path if it is not used by any other instance in the cluster
				delPaths.Insert(dataDir)
			}
		}

		// For TiFlash, we need to delete storage.remote.cache.dir
		if ins.ComponentName() == spec.ComponentTiFlash {
			tiflashInstance := ins.(*spec.TiFlashInstance)
			tiflashSpec := tiflashInstance.InstanceSpec.(*spec.TiFlashSpec)
			if remoteCacheDir, ok := tiflashSpec.Config[spec.TiFlashRemoteCacheDir]; ok {
				delPaths.Insert(remoteCacheDir.(string))
			}
		}

		logDir := ins.LogDir()

		// In TiDB-Ansible, deploy dir are shared by all components on the same
		// host, so not deleting it.
		if ins.IsImported() {
			// not deleting files for imported clusters
			if !strings.HasPrefix(logDir, ins.DeployDir()) && cls.CountDir(ins.GetManageHost(), logDir) == 1 {
				delPaths.Insert(logDir)
			}
			logger.Warnf("Deploy dir %s not deleted for TiDB-Ansible imported instance %s.",
				ins.DeployDir(), ins.InstanceName())
		} else {
			if keepDeployDir {
				delPaths.Insert(filepath.Join(deployDir, "conf"))
				delPaths.Insert(filepath.Join(deployDir, "bin"))
				delPaths.Insert(filepath.Join(deployDir, "scripts"))
				if cls.BaseTopo().GlobalOptions.TLSEnabled {
					delPaths.Insert(filepath.Join(deployDir, spec.TLSCertKeyDir))
				}
				// only delete path if it is not used by any other instance in the cluster
				if strings.HasPrefix(logDir, deployDir) && cls.CountDir(ins.GetManageHost(), logDir) == 1 {
					delPaths.Insert(logDir)
				}
			} else {
				// only delete path if it is not used by any other instance in the cluster
				if cls.CountDir(ins.GetManageHost(), logDir) == 1 {
					delPaths.Insert(logDir)
				}
				if cls.CountDir(ins.GetManageHost(), ins.DeployDir()) == 1 {
					delPaths.Insert(ins.DeployDir())
				}
			}
		}

		// check for deploy dir again, to avoid unused files being left on disk
		dpCnt := 0
		for _, dir := range delPaths.Slice() {
			if strings.HasPrefix(dir, deployDir+"/") { // only check subdir of deploy dir
				dpCnt++
			}
		}
		if !ins.IsImported() && cls.CountDir(ins.GetManageHost(), deployDir)-dpCnt == 1 {
			delPaths.Insert(deployDir)
		}

		systemdDir := "/etc/systemd/system/"
		sudo := true
		if cls.BaseTopo().GlobalOptions.SystemdMode == spec.UserMode {
			systemdDir = "~/.config/systemd/user/"
			sudo = false
		}

		if svc := ins.ServiceName(); svc != "" {
			delPaths.Insert(fmt.Sprintf("%s%s", systemdDir, svc))
		}
		logger.Debugf("Deleting paths on %s: %s", ins.GetManageHost(), strings.Join(delPaths.Slice(), " "))
		for _, delPath := range delPaths.Slice() {
			c := module.ShellModuleConfig{
				Command:  fmt.Sprintf("rm -rf %s;", delPath),
				Sudo:     sudo, // the .service files are in a directory owned by root
				Chdir:    "",
				UseShell: false,
			}
			shell := module.NewShellModule(c)
			_, _, err := shell.Execute(ctx, e)

			if err != nil {
				// Ignore error and continue.For example, deleting a mount point will result in a "Device or resource busy" error.
				logger.Warnf("%s", color.YellowString("Warn: failed to delete path \"%s\" on %s.Please check this error message and manually delete if necessary.\nerrmsg: %s", delPath, ins.GetManageHost(), err))
			}
		}

		logger.Infof("Destroy %s finished", ins.GetManageHost())
		logger.Infof("- Destroy %s paths: %v", ins.ComponentName(), delPaths.Slice())
	}

	return nil
}

// DestroyTombstone remove the tombstone node in spec and destroy them.
// If returNodesOnly is true, it will only return the node id that can be destroy.
func DestroyTombstone(
	ctx context.Context,
	cluster *spec.Specification,
	returNodesOnly bool,
	options Options,
	tlsCfg *tls.Config,
) (nodes []string, err error) {
	return DestroyClusterTombstone(ctx, cluster, returNodesOnly, options, tlsCfg)
}

// DestroyClusterTombstone remove the tombstone node in spec and destroy them.
// If returNodesOnly is true, it will only return the node id that can be destroy.
func DestroyClusterTombstone(
	ctx context.Context,
	cluster *spec.Specification,
	returNodesOnly bool,
	options Options,
	tlsCfg *tls.Config,
) (nodes []string, err error) {
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	instCount := map[string]int{}
	for _, component := range cluster.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			instCount[instance.GetManageHost()]++
		}
	}

	var pdEndpoints []string
	forcePDEndpoints := os.Getenv(EnvNamePDEndpointOverwrite) // custom set PD endpoint list

	if forcePDEndpoints != "" {
		pdEndpoints = strings.Split(forcePDEndpoints, ",")
		logger.Warnf("%s is set, using %s as PD endpoints", EnvNamePDEndpointOverwrite, pdEndpoints)
	} else {
		pdEndpoints = cluster.GetPDListWithManageHost()
	}

	var pdClient = api.NewPDClient(ctx, pdEndpoints, 10*time.Second, tlsCfg)

	tcpProxy := proxy.GetTCPProxy()
	if tcpProxy != nil {
		closeC := tcpProxy.Run(pdEndpoints)
		defer tcpProxy.Close(closeC)
		pdEndpoints = tcpProxy.GetEndpoints()
	}
	binlogClient, err := api.NewBinlogClient(pdEndpoints, 5*time.Second, tlsCfg)
	if err != nil {
		return nil, err
	}

	filterID := func(instance []spec.Instance, id string) (res []spec.Instance) {
		for _, ins := range instance {
			if ins.ID() == id {
				res = append(res, ins)
			}
		}
		return
	}

	maybeDestroyMonitor := func(instances []spec.Instance, id string) error {
		instances = filterID(instances, id)

		for _, instance := range instances {
			instCount[instance.GetManageHost()]--
			err := StopAndDestroyInstance(ctx, cluster, instance, options, true, instCount[instance.GetManageHost()] == 0, tlsCfg)
			if err != nil {
				if !options.Force {
					return err
				}
				logger.Warnf("failed to stop and destroy instance %s (%s), ignored as --force is set, you may need to manually cleanup the files",
					instance, err)
			}
		}
		return nil
	}

	var kvServers []*spec.TiKVSpec
	for _, s := range cluster.TiKVServers {
		if !s.Offline {
			kvServers = append(kvServers, s)
			continue
		}

		id := utils.JoinHostPort(s.Host, s.Port)

		tombstone, err := pdClient.IsTombStone(id)
		if err != nil {
			return nil, err
		}

		if !tombstone {
			kvServers = append(kvServers, s)
			continue
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.TiKVComponent{Topology: cluster}).Instances()
		if err := maybeDestroyMonitor(instances, id); err != nil {
			return nil, err
		}
	}

	var flashServers []*spec.TiFlashSpec
	for _, s := range cluster.TiFlashServers {
		if !s.Offline {
			flashServers = append(flashServers, s)
			continue
		}

		id := utils.JoinHostPort(s.Host, s.FlashServicePort)

		tombstone, err := pdClient.IsTombStone(id)
		if err != nil {
			return nil, err
		}

		if !tombstone {
			flashServers = append(flashServers, s)
			continue
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.TiFlashComponent{Topology: cluster}).Instances()
		id = utils.JoinHostPort(s.Host, s.GetMainPort())
		if err := maybeDestroyMonitor(instances, id); err != nil {
			return nil, err
		}
	}

	var pumpServers []*spec.PumpSpec
	for _, s := range cluster.PumpServers {
		if !s.Offline {
			pumpServers = append(pumpServers, s)
			continue
		}

		id := utils.JoinHostPort(s.Host, s.Port)

		tombstone, err := binlogClient.IsPumpTombstone(ctx, id)
		if err != nil {
			return nil, err
		}

		if !tombstone {
			pumpServers = append(pumpServers, s)
			continue
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.PumpComponent{Topology: cluster}).Instances()
		if err := maybeDestroyMonitor(instances, id); err != nil {
			return nil, err
		}
	}

	var drainerServers []*spec.DrainerSpec
	for _, s := range cluster.Drainers {
		if !s.Offline {
			drainerServers = append(drainerServers, s)
			continue
		}

		id := utils.JoinHostPort(s.Host, s.Port)

		tombstone, err := binlogClient.IsDrainerTombstone(ctx, id)
		if err != nil {
			return nil, err
		}

		if !tombstone {
			drainerServers = append(drainerServers, s)
			continue
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.DrainerComponent{Topology: cluster}).Instances()
		if err := maybeDestroyMonitor(instances, id); err != nil {
			return nil, err
		}
	}

	if returNodesOnly {
		return
	}

	cluster.TiKVServers = kvServers
	cluster.TiFlashServers = flashServers
	cluster.PumpServers = pumpServers
	cluster.Drainers = drainerServers

	return
}
