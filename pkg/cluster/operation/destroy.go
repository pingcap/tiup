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
	"path"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
)

// Cleanup the cluster
func Cleanup(
	getter ExecutorGetter,
	cluster spec.Topology,
	options Options,
) error {
	coms := cluster.ComponentsByStopOrder()

	instCount := map[string]int{}
	cluster.IterInstance(func(inst spec.Instance) {
		instCount[inst.GetHost()] = instCount[inst.GetHost()] + 1
	})

	for _, com := range coms {
		insts := com.Instances()
		if err := CleanupComponent(getter, insts, cluster, options); err != nil {
			return errors.Annotatef(err, "failed to cleanup %s", com.Name())
		}
	}

	return nil
}

// Destroy the cluster.
func Destroy(
	getter ExecutorGetter,
	cluster spec.Topology,
	options Options,
) error {
	uniqueHosts := set.NewStringSet()
	coms := cluster.ComponentsByStopOrder()

	instCount := map[string]int{}
	cluster.IterInstance(func(inst spec.Instance) {
		instCount[inst.GetHost()] = instCount[inst.GetHost()] + 1
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

	// Delete all global deploy directory
	for host := range uniqueHosts {
		if err := DeleteGlobalDirs(getter, host, cluster.BaseTopo().GlobalOptions); err != nil {
			return nil
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
func DestroyMonitored(getter ExecutorGetter, inst spec.Instance, options *spec.MonitoredOptions, timeout int64) error {
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
func CleanupComponent(getter ExecutorGetter, instances []spec.Instance, cls spec.Topology, options Options) error {
	retainDataRoles := set.NewStringSet(options.RetainDataRoles...)
	retainDataNodes := set.NewStringSet(options.RetainDataNodes...)

	for _, ins := range instances {
		// Some data of instances will be retained
		dataRetained := retainDataRoles.Exist(ins.ComponentName()) ||
			retainDataNodes.Exist(ins.ID()) || retainDataNodes.Exist(ins.GetHost())

		if dataRetained {
			continue
		}

		e := getter.Get(ins.GetHost())
		log.Infof("Cleanup instance %s", ins.GetHost())

		delFiles := set.NewStringSet()

		if options.CleanupData && len(ins.DataDir()) > 0 {
			for _, dataDir := range strings.Split(ins.DataDir(), ",") {
				delFiles.Insert(path.Join(dataDir, "*"))
			}
		}

		if options.CleanupLog && len(ins.LogDir()) > 0 {
			for _, logDir := range strings.Split(ins.LogDir(), ",") {
				delFiles.Insert(path.Join(logDir, "*"))
			}
		}

		log.Debugf("Deleting paths on %s: %s", ins.GetHost(), strings.Join(delFiles.Slice(), " "))
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
			return errors.Annotatef(err, "failed to cleanup: %s", ins.GetHost())
		}

		log.Infof("Cleanup %s success", ins.GetHost())
		log.Infof("- Clanup %s files: %v", ins.ComponentName(), delFiles.Slice())
	}

	return nil
}

// DestroyComponent destroy the instances.
func DestroyComponent(getter ExecutorGetter, instances []spec.Instance, cls spec.Topology, options Options) error {
	if len(instances) <= 0 {
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
		if cls.CountDir(ins.GetHost(), deployDir)-dpCnt == 1 {
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
