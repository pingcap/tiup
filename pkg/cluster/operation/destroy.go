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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pingcap/errors"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
)

// Destroy the cluster.
func Destroy(
	getter ExecutorGetter,
	cluster spec.Topology,
	options Options,
) error {
	coms := cluster.ComponentsByStopOrder()

	instCount := map[string]int{}
	cluster.IterInstance(func(inst spec.Instance) {
		instCount[inst.GetHost()]++
	})

	for _, com := range coms {
		insts := com.Instances()
		err := DestroyComponent(getter, insts, cluster, options)
		if err != nil && !options.Force {
			return errors.Annotatef(err, "failed to destroy %s", com.Name())
		}
		for _, inst := range insts {
			instCount[inst.GetHost()]--
			if instCount[inst.GetHost()] == 0 {
				if cluster.GetMonitoredOptions() != nil {
					if err := DestroyMonitored(getter, inst, cluster.GetMonitoredOptions(), options.OptTimeout); err != nil && !options.Force {
						return err
					}
				}
			}
		}
	}

	gOpts := cluster.BaseTopo().GlobalOptions

	// Delete all global deploy directory
	for host := range instCount {
		if err := DeleteGlobalDirs(getter, host, gOpts); err != nil {
			return nil
		}
	}

	// after all things done, try to remove SSH public key
	for host := range instCount {
		if err := DeletePublicKey(getter, host); err != nil {
			return nil
		}
	}

	return nil
}

// StopAndDestroyInstance stop and destroy the instance,
// if this instance is the host's last one, and the host has monitor deployed,
// we need to destroy the monitor, either
func StopAndDestroyInstance(getter ExecutorGetter, cluster spec.Topology, instance spec.Instance, options Options, destroyNode bool) error {
	ignoreErr := options.Force
	compName := instance.ComponentName()

	// just try to stop and destroy
	if err := StopComponent(getter, []spec.Instance{instance}, options.OptTimeout); err != nil {
		if !ignoreErr {
			return errors.Annotatef(err, "failed to stop %s", compName)
		}
		log.Warnf("failed to stop %s: %v", compName, err)
	}
	if err := DestroyComponent(getter, []spec.Instance{instance}, cluster, options); err != nil {
		if !ignoreErr {
			return errors.Annotatef(err, "failed to destroy %s", compName)
		}
		log.Warnf("failed to destroy %s: %v", compName, err)
	}

	if destroyNode {
		// monitoredOptions for dm cluster is nil
		monitoredOptions := cluster.GetMonitoredOptions()

		if monitoredOptions != nil {
			if err := StopMonitored(getter, instance, monitoredOptions, options.OptTimeout); err != nil {
				if !ignoreErr {
					return errors.Annotatef(err, "failed to stop monitor")
				}
				log.Warnf("failed to stop %s: %v", "monitor", err)
			}
			if err := DestroyMonitored(getter, instance, monitoredOptions, options.OptTimeout); err != nil {
				if !ignoreErr {
					return errors.Annotatef(err, "failed to destroy monitor")
				}
				log.Warnf("failed to destroy %s: %v", "monitor", err)
			}
		}

		if err := DeletePublicKey(getter, instance.GetHost()); err != nil {
			if !ignoreErr {
				return errors.Annotatef(err, "failed to delete public key")
			}
			log.Warnf("failed to delete public key")
		}
	}
	return nil
}

// DeleteGlobalDirs deletes all global directories if they are empty
func DeleteGlobalDirs(getter ExecutorGetter, host string, options *spec.GlobalOptions) error {
	if options == nil {
		return nil
	}

	e := getter.Get(host)
	log.Infof("Clean global directories %s", host)
	for _, dir := range []string{options.LogDir, options.DeployDir, options.DataDir} {
		if dir == "" {
			continue
		}
		dir = spec.Abs(options.User, dir)

		log.Infof("\tClean directory %s on instance %s", dir, host)

		c := module.ShellModuleConfig{
			Command:  fmt.Sprintf("rmdir %s > /dev/null 2>&1 || true", dir),
			Chdir:    "",
			UseShell: false,
		}
		shell := module.NewShellModule(c)
		stdout, stderr, err := shell.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to clean directory %s on: %s", dir, host)
		}
	}

	log.Infof("Clean global directories %s success", host)
	return nil
}

// DeletePublicKey deletes the SSH public key from host
func DeletePublicKey(getter ExecutorGetter, host string) error {
	e := getter.Get(host)
	log.Infof("Delete public key %s", host)
	_, pubKeyPath := getter.GetSSHKeySet()
	publicKey, err := ioutil.ReadFile(pubKeyPath)
	if err != nil {
		return perrs.Trace(err)
	}

	pubKey := string(bytes.TrimSpace(publicKey))
	pubKey = strings.ReplaceAll(pubKey, "/", "\\/")
	pubKeysFile := executor.FindSSHAuthorizedKeysFile(e)

	// delete the public key with Linux `sed` toolkit
	c := module.ShellModuleConfig{
		Command:  fmt.Sprintf("sed -i '/%s/d' %s", pubKey, pubKeysFile),
		UseShell: false,
	}
	shell := module.NewShellModule(c)
	stdout, stderr, err := shell.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 {
		log.Errorf(string(stderr))
	}

	if err != nil {
		return errors.Annotatef(err, "failed to delete pulblic key on: %s", host)
	}

	log.Infof("Delete public key %s success", host)
	return nil
}

// DestroyMonitored destroy the monitored service.
func DestroyMonitored(getter ExecutorGetter, inst spec.Instance, options *spec.MonitoredOptions, timeout uint64) error {
	e := getter.Get(inst.GetHost())
	log.Infof("Destroying monitored %s", inst.GetHost())

	log.Infof("\tDestroying instance %s", inst.GetHost())

	// Stop by systemd.
	delPaths := make([]string, 0)

	delPaths = append(delPaths, options.DataDir)
	delPaths = append(delPaths, options.LogDir)

	// In TiDB-Ansible, deploy dir are shared by all components on the same
	// host, so not deleting it.
	if !inst.IsImported() {
		delPaths = append(delPaths, options.DeployDir)
	} else {
		log.Warnf("Monitored deploy dir %s not deleted for TiDB-Ansible imported instance %s.",
			options.DeployDir, inst.InstanceName())
	}

	delPaths = append(delPaths, fmt.Sprintf("/etc/systemd/system/%s-%d.service", spec.ComponentNodeExporter, options.NodeExporterPort))
	delPaths = append(delPaths, fmt.Sprintf("/etc/systemd/system/%s-%d.service", spec.ComponentBlackboxExporter, options.BlackboxExporterPort))

	c := module.ShellModuleConfig{
		Command:  fmt.Sprintf("rm -rf %s;", strings.Join(delPaths, " ")),
		Sudo:     true, // the .service files are in a directory owned by root
		Chdir:    "",
		UseShell: false,
	}
	shell := module.NewShellModule(c)
	stdout, stderr, err := shell.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 {
		log.Errorf(string(stderr))
	}

	if err != nil {
		return errors.Annotatef(err, "failed to destroy monitored: %s", inst.GetHost())
	}

	if err := spec.PortStopped(e, options.NodeExporterPort, timeout); err != nil {
		str := fmt.Sprintf("%s failed to destroy node exportoer: %s", inst.GetHost(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}
	if err := spec.PortStopped(e, options.BlackboxExporterPort, timeout); err != nil {
		str := fmt.Sprintf("%s failed to destroy blackbox exportoer: %s", inst.GetHost(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}

	log.Infof("Destroy monitored on %s success", inst.GetHost())

	return nil
}

// CleanupComponent cleanup the instances
func CleanupComponent(getter ExecutorGetter, delFileMaps map[string]set.StringSet) error {
	for host, delFiles := range delFileMaps {
		e := getter.Get(host)
		log.Infof("Cleanup instance %s", host)
		log.Debugf("Deleting paths on %s: %s", host, strings.Join(delFiles.Slice(), " "))
		c := module.ShellModuleConfig{
			Command:  fmt.Sprintf("rm -rf %s;", strings.Join(delFiles.Slice(), " ")),
			Sudo:     true, // the .service files are in a directory owned by root
			Chdir:    "",
			UseShell: true,
		}
		shell := module.NewShellModule(c)
		stdout, stderr, err := shell.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to cleanup: %s", host)
		}

		log.Infof("Cleanup %s success", host)
	}

	return nil
}

// DestroyComponent destroy the instances.
func DestroyComponent(getter ExecutorGetter, instances []spec.Instance, cls spec.Topology, options Options) error {
	if len(instances) == 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Destroying component %s", name)

	retainDataRoles := set.NewStringSet(options.RetainDataRoles...)
	retainDataNodes := set.NewStringSet(options.RetainDataNodes...)

	for _, ins := range instances {
		// Some data of instances will be retained
		dataRetained := retainDataRoles.Exist(ins.ComponentName()) ||
			retainDataNodes.Exist(ins.ID()) || retainDataNodes.Exist(ins.GetHost())

		e := getter.Get(ins.GetHost())
		log.Infof("Destroying instance %s", ins.GetHost())

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
			if !dataRetained && cls.CountDir(ins.GetHost(), dataDir) == 1 {
				// only delete path if it is not used by any other instance in the cluster
				delPaths.Insert(dataDir)
			}
		}

		logDir := ins.LogDir()

		// In TiDB-Ansible, deploy dir are shared by all components on the same
		// host, so not deleting it.
		if ins.IsImported() {
			// not deleting files for imported clusters
			if !strings.HasPrefix(logDir, ins.DeployDir()) && cls.CountDir(ins.GetHost(), logDir) == 1 {
				delPaths.Insert(logDir)
			}
			log.Warnf("Deploy dir %s not deleted for TiDB-Ansible imported instance %s.",
				ins.DeployDir(), ins.InstanceName())
		} else {
			if keepDeployDir {
				delPaths.Insert(filepath.Join(deployDir, "conf"))
				delPaths.Insert(filepath.Join(deployDir, "bin"))
				delPaths.Insert(filepath.Join(deployDir, "scripts"))
				if cls.BaseTopo().GlobalOptions.TLSEnabled {
					delPaths.Insert(filepath.Join(deployDir, "tls"))
				}
				// only delete path if it is not used by any other instance in the cluster
				if strings.HasPrefix(logDir, deployDir) && cls.CountDir(ins.GetHost(), logDir) == 1 {
					delPaths.Insert(logDir)
				}
			} else {
				// only delete path if it is not used by any other instance in the cluster
				if cls.CountDir(ins.GetHost(), logDir) == 1 {
					delPaths.Insert(logDir)
				}
				if cls.CountDir(ins.GetHost(), ins.DeployDir()) == 1 {
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
		if !ins.IsImported() && cls.CountDir(ins.GetHost(), deployDir)-dpCnt == 1 {
			delPaths.Insert(deployDir)
		}

		if svc := ins.ServiceName(); svc != "" {
			delPaths.Insert(fmt.Sprintf("/etc/systemd/system/%s", svc))
		}
		log.Debugf("Deleting paths on %s: %s", ins.GetHost(), strings.Join(delPaths.Slice(), " "))
		c := module.ShellModuleConfig{
			Command:  fmt.Sprintf("rm -rf %s;", strings.Join(delPaths.Slice(), " ")),
			Sudo:     true, // the .service files are in a directory owned by root
			Chdir:    "",
			UseShell: false,
		}
		shell := module.NewShellModule(c)
		stdout, stderr, err := shell.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to destroy: %s", ins.GetHost())
		}

		log.Infof("Destroy %s success", ins.GetHost())
		log.Infof("- Destroy %s paths: %v", ins.ComponentName(), delPaths.Slice())
	}

	return nil
}

// DestroyTombstone remove the tombstone node in spec and destroy them.
// If returNodesOnly is true, it will only return the node id that can be destroy.
func DestroyTombstone(
	getter ExecutorGetter,
	cluster *spec.Specification,
	returNodesOnly bool,
	options Options,
	tlsCfg *tls.Config,
) (nodes []string, err error) {
	return DestroyClusterTombstone(getter, cluster, returNodesOnly, options, tlsCfg)
}

// DestroyClusterTombstone remove the tombstone node in spec and destroy them.
// If returNodesOnly is true, it will only return the node id that can be destroy.
func DestroyClusterTombstone(
	getter ExecutorGetter,
	cluster *spec.Specification,
	returNodesOnly bool,
	options Options,
	tlsCfg *tls.Config,
) (nodes []string, err error) {
	instCount := map[string]int{}
	for _, component := range cluster.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			instCount[instance.GetHost()]++
		}
	}

	var pdClient = api.NewPDClient(cluster.GetPDList(), 10*time.Second, tlsCfg)

	binlogClient, err := api.NewBinlogClient(cluster.GetPDList(), tlsCfg)
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
			instCount[instance.GetHost()]--
			err := StopAndDestroyInstance(getter, cluster, instance, options, instCount[instance.GetHost()] == 0)
			if err != nil {
				return err
			}
		}
		return nil
	}

	var kvServers []spec.TiKVSpec
	for _, s := range cluster.TiKVServers {
		if !s.Offline {
			kvServers = append(kvServers, s)
			continue
		}

		id := s.Host + ":" + strconv.Itoa(s.Port)

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

	var flashServers []spec.TiFlashSpec
	for _, s := range cluster.TiFlashServers {
		if !s.Offline {
			flashServers = append(flashServers, s)
			continue
		}

		id := s.Host + ":" + strconv.Itoa(s.FlashServicePort)

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
		id = s.Host + ":" + strconv.Itoa(s.GetMainPort())
		if err := maybeDestroyMonitor(instances, id); err != nil {
			return nil, err
		}
	}

	var pumpServers []spec.PumpSpec
	for _, s := range cluster.PumpServers {
		if !s.Offline {
			pumpServers = append(pumpServers, s)
			continue
		}

		id := s.Host + ":" + strconv.Itoa(s.Port)

		tombstone, err := binlogClient.IsPumpTombstone(id)
		if err != nil {
			return nil, err
		}

		if !tombstone {
			pumpServers = append(pumpServers, s)
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

	var drainerServers []spec.DrainerSpec
	for _, s := range cluster.Drainers {
		if !s.Offline {
			drainerServers = append(drainerServers, s)
			continue
		}

		id := s.Host + ":" + strconv.Itoa(s.Port)

		tombstone, err := binlogClient.IsDrainerTombstone(id)
		if err != nil {
			return nil, err
		}

		if !tombstone {
			drainerServers = append(drainerServers, s)
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
