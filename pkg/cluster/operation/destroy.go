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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
)

// Destroy the cluster.
func Destroy(
	getter ExecutorGetter,
	spec meta.Specification,
	options Options,
) error {
	uniqueHosts := set.NewStringSet()
	coms := spec.ComponentsByStopOrder()

	instCount := map[string]int{}
	spec.IterInstance(func(inst meta.Instance) {
		instCount[inst.GetHost()] = instCount[inst.GetHost()] + 1
	})

	for _, com := range coms {
		insts := com.Instances()
		err := DestroyComponent(getter, insts, options)
		if err != nil {
			return errors.Annotatef(err, "failed to destroy %s", com.Name())
		}
		if clusterSpec := spec.GetClusterSpecification(); clusterSpec != nil {
			for _, inst := range insts {
				instCount[inst.GetHost()]--
				if instCount[inst.GetHost()] == 0 {
					if err := DestroyMonitored(getter, inst, clusterSpec.MonitoredOptions, options.OptTimeout); err != nil {
						return err
					}
				}
			}
		}
	}

	// Delete all global deploy directory
	for host := range uniqueHosts {
		if err := DeleteGlobalDirs(getter, host, spec.GetGlobalOptions()); err != nil {
			return nil
		}
	}

	return nil
}

// DeleteGlobalDirs deletes all global directory if them empty
func DeleteGlobalDirs(getter ExecutorGetter, host string, options meta.GlobalOptions) error {
	e := getter.Get(host)
	log.Infof("Clean global directories %s", host)
	for _, dir := range []string{options.LogDir, options.DeployDir, options.DataDir} {
		if dir == "" {
			continue
		}
		dir = clusterutil.Abs(options.User, dir)

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

// DestroyMonitored destroy the monitored service.
func DestroyMonitored(getter ExecutorGetter, inst meta.Instance, options meta.MonitoredOptions, timeout int64) error {
	e := getter.Get(inst.GetHost())
	log.Infof("Destroying monitored %s", inst.GetHost())

	log.Infof("Destroying monitored")
	log.Infof("\tDestroying instance %s", inst.GetHost())

	// Stop by systemd.
	delPaths := make([]string, 0)

	delPaths = append(delPaths, options.DataDir)
	delPaths = append(delPaths, options.LogDir)

	// In TiDB-Ansible, deploy dir are shared by all components on the same
	// host, so not deleting it.
	// TODO: this may leave undeleted files when destroying the cluster, fix
	// that later.
	if !inst.IsImported() {
		delPaths = append(delPaths, options.DeployDir)
	} else {
		log.Warnf("Monitored deploy dir %s not deleted for TiDB-Ansible imported instance %s.",
			options.DeployDir, inst.InstanceName())
	}

	delPaths = append(delPaths, fmt.Sprintf("/etc/systemd/system/%s-%d.service", meta.ComponentNodeExporter, options.NodeExporterPort))
	delPaths = append(delPaths, fmt.Sprintf("/etc/systemd/system/%s-%d.service", meta.ComponentBlackboxExporter, options.BlackboxExporterPort))

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

	if err := meta.PortStopped(e, options.NodeExporterPort, timeout); err != nil {
		str := fmt.Sprintf("%s failed to destroy node exportoer: %s", inst.GetHost(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}
	if err := meta.PortStopped(e, options.BlackboxExporterPort, timeout); err != nil {
		str := fmt.Sprintf("%s failed to destroy blackbox exportoer: %s", inst.GetHost(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}

	log.Infof("Destroy monitored on %s success", inst.GetHost())

	return nil
}

// DestroyComponent destroy the instances.
func DestroyComponent(getter ExecutorGetter, instances []meta.Instance, options Options) error {
	if len(instances) <= 0 {
		return nil
	}

	timeout := options.OptTimeout
	name := instances[0].ComponentName()
	log.Infof("Destroying component %s", name)

	retainDataRoles := set.NewStringSet(options.RetainDataRoles...)
	retainDataNodes := set.NewStringSet(options.RetainDataNodes...)

	for _, ins := range instances {
		// Some data of instances will be retained
		dataRetained := (len(retainDataRoles) > 0 && retainDataRoles.Exist(ins.ComponentName())) ||
			(len(retainDataNodes) > 0 && retainDataNodes.Exist(ins.ID()))

		e := getter.Get(ins.GetHost())
		log.Infof("Destroying instance %s", ins.GetHost())

		// Retain the deploy directory if the users want to retain the data directory
		// and the data directory is a sub-directory of deploy directory
		keepDeployDir := false
		deployDir := ins.DeployDir()

		// Stop by systemd.
		var delPaths []string
		switch name {
		case meta.ComponentTiKV, meta.ComponentPD, meta.ComponentPump, meta.ComponentDrainer, meta.ComponentPrometheus, meta.ComponentAlertManager, meta.ComponentDMMaster, meta.ComponentDMWorker, meta.ComponentDMPortal:
			if !dataRetained {
				delPaths = append(delPaths, ins.DataDir())
			} else if strings.HasPrefix(ins.DataDir(), deployDir) {
				keepDeployDir = true
			}
		case meta.ComponentTiFlash:
			for _, dataDir := range strings.Split(ins.DataDir(), ",") {
				if !dataRetained {
					delPaths = append(delPaths, dataDir)
				} else if strings.HasPrefix(dataDir, deployDir) {
					keepDeployDir = true
				}
			}
		}

		// check if service is down before deleting files
		if err := ins.WaitForDown(e, timeout); err != nil {
			str := fmt.Sprintf("%s error destroying %s: %s", ins.GetHost(), ins.ComponentName(), err)
			log.Errorf(str)
			if !clusterutil.IsTimeoutOrMaxRetry(err) {
				return errors.Annotatef(err, str)
			}
			log.Warnf("You may manually check if the process on %s:%d is still running", ins.GetHost(), ins.GetPort())
		}

		// In TiDB-Ansible, deploy dir are shared by all components on the same
		// host, so not deleting it.
		// TODO: this may leave undeleted files when destroying the cluster, fix
		// that later.
		if !ins.IsImported() {
			if keepDeployDir {
				delPaths = append(delPaths,
					filepath.Join(deployDir, "conf"),
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "scripts"),
				)
				if logDir := ins.LogDir(); strings.HasPrefix(logDir, deployDir) {
					delPaths = append(delPaths, logDir)
				}
			} else {
				delPaths = append(delPaths, deployDir)
				if logDir := ins.LogDir(); !strings.HasPrefix(logDir, deployDir) {
					delPaths = append(delPaths, logDir)
				}
			}
		} else {
			log.Warnf("Deploy dir %s not deleted for TiDB-Ansible imported instance %s.",
				ins.DeployDir(), ins.InstanceName())
		}
		delPaths = append(delPaths, fmt.Sprintf("/etc/systemd/system/%s", ins.ServiceName()))
		log.Debugf("Deleting paths on %s: %s", ins.GetHost(), strings.Join(delPaths, " "))
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
			return errors.Annotatef(err, "failed to destroy: %s", ins.GetHost())
		}

		log.Infof("Destroy %s success", ins.GetHost())
		log.Infof("- Destroy %s paths: %v", ins.ComponentName(), delPaths)
	}

	return nil
}
