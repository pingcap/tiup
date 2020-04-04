package operator

import (
	"fmt"
	"strings"

	"github.com/pingcap-incubator/tiup-cluster/pkg/clusterutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/module"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
)

// Destroy the cluster.
func Destroy(
	getter ExecutorGetter,
	spec *meta.Specification,
) error {
	uniqueHosts := set.NewStringSet()
	coms := spec.ComponentsByStopOrder()

	instCount := map[string]int{}
	spec.IterInstance(func(inst meta.Instance) {
		instCount[inst.GetHost()] = instCount[inst.GetHost()] + 1
	})

	for _, com := range coms {
		insts := com.Instances()
		err := DestroyComponent(getter, insts)
		if err != nil {
			return errors.Annotatef(err, "failed to destroy %s", com.Name())
		}
		for _, inst := range insts {
			instCount[inst.GetHost()]--
			if instCount[inst.GetHost()] == 0 {
				if err := DestroyMonitored(getter, inst, spec.MonitoredOptions); err != nil {
					return err
				}
			}
		}
	}

	// Delete all global deploy directory
	for host := range uniqueHosts {
		if err := DeleteGlobalDirs(getter, host, spec.GlobalOptions); err != nil {
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
func DestroyMonitored(getter ExecutorGetter, inst meta.Instance, options meta.MonitoredOptions) error {
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

	if err := meta.PortStopped(e, options.NodeExporterPort); err != nil {
		str := fmt.Sprintf("%s failed to destroy node exportoer: %s", inst.GetHost(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}
	if err := meta.PortStopped(e, options.BlackboxExporterPort); err != nil {
		str := fmt.Sprintf("%s failed to destroy blackbox exportoer: %s", inst.GetHost(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}

	log.Infof("Destroy monitored on %s success", inst.GetHost())

	return nil
}

// DestroyComponent destroy the instances.
func DestroyComponent(getter ExecutorGetter, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Destroying component %s", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		log.Infof("Destroying instance %s", ins.GetHost())

		// Stop by systemd.
		delPaths := make([]string, 0)
		switch name {
		case meta.ComponentTiKV, meta.ComponentPD, meta.ComponentPump, meta.ComponentDrainer, meta.ComponentPrometheus, meta.ComponentAlertManager:
			delPaths = append(delPaths, ins.DataDir())
		}

		// In TiDB-Ansible, deploy dir are shared by all components on the same
		// host, so not deleting it.
		// TODO: this may leave undeleted files when destroying the cluster, fix
		// that later.
		if !ins.IsImported() {
			delPaths = append(delPaths, ins.DeployDir())
			if logDir := ins.LogDir(); !strings.HasPrefix(ins.DeployDir(), logDir) {
				delPaths = append(delPaths, logDir)
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

		err = ins.WaitForDown(e)
		if err != nil {
			str := fmt.Sprintf("%s failed to destroy: %s", ins.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log.Infof("Destroy %s success", ins.GetHost())
	}

	return nil
}
