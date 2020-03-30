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
	"strings"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/module"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
)

// Start the cluster.
func Start(
	getter ExecutorGetter,
	spec *meta.Specification,
	options Options,
) error {
	uniqueHosts := set.NewStringSet()
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := spec.ComponentsByStartOrder()
	components = filterComponent(components, roleFilter)

	for _, com := range components {
		insts := filterInstance(com.Instances(), nodeFilter)
		err := StartComponent(getter, insts)
		if err != nil {
			return errors.Annotatef(err, "failed to start %s", com.Name())
		}
		for _, inst := range insts {
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				if err := StartMonitored(getter, inst, spec.MonitoredOptions); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Stop the cluster.
func Stop(
	getter ExecutorGetter,
	spec *meta.Specification,
	options Options,
) error {
	uniqueHosts := set.NewStringSet()
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := spec.ComponentsByStopOrder()
	components = filterComponent(components, roleFilter)

	for _, com := range components {
		insts := filterInstance(com.Instances(), nodeFilter)
		err := StopComponent(getter, insts)
		if err != nil {
			return errors.Annotatef(err, "failed to stop %s", com.Name())
		}
		for _, inst := range insts {
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				if err := StopMonitored(getter, inst, spec.MonitoredOptions); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Restart the cluster.
func Restart(
	getter ExecutorGetter,
	spec *meta.Specification,
	options Options,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	components := spec.ComponentsByStartOrder()
	components = filterComponent(components, roleFilter)

	err := Stop(getter, spec, options)
	if err != nil {
		return errors.Annotatef(err, "failed to stop")
	}

	err = Start(getter, spec, options)
	if err != nil {
		return errors.Annotatef(err, "failed to start")
	}

	return nil
}

// StartMonitored start BlackboxExporter and NodeExporter
func StartMonitored(getter ExecutorGetter, instance meta.Instance, options meta.MonitoredOptions) error {
	ports := map[string]int{
		meta.ComponentNodeExporter:     options.NodeExporterPort,
		meta.ComponentBlackboxExporter: options.BlackboxExporterPort,
	}
	e := getter.Get(instance.GetHost())
	for _, comp := range []string{meta.ComponentNodeExporter, meta.ComponentBlackboxExporter} {
		log.Infof("Starting component %s", comp)
		log.Infof("\tStarting instance %s", instance.GetHost())
		c := module.SystemdModuleConfig{
			Unit:         fmt.Sprintf("%s-%d.service", comp, ports[comp]),
			ReloadDaemon: true,
			Action:       "start",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			log.Output(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to start: %s", instance.GetHost())
		}

		// Check ready.
		if err := meta.PortStarted(e, ports[comp]); err != nil {
			str := fmt.Sprintf("\t%s failed to start: %s", instance.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log.Infof("\tStart %s success", instance.GetHost())
	}

	return nil
}

// StartComponent start the instances.
func StartComponent(getter ExecutorGetter, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Starting component %s", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		log.Infof("\tStarting instance %s", ins.GetHost())

		// Start by systemd.
		c := module.SystemdModuleConfig{
			Unit:         ins.ServiceName(),
			ReloadDaemon: true,
			Action:       "start",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			log.Output(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to start: %s", ins.GetHost())
		}

		// Check ready.
		err = ins.Ready(e)
		if err != nil {
			str := fmt.Sprintf("\t%s failed to start: %s", ins.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log.Infof("\tStart %s success", ins.GetHost())
	}

	return nil
}

// StopMonitored stop BlackboxExporter and NodeExporter
func StopMonitored(getter ExecutorGetter, instance meta.Instance, options meta.MonitoredOptions) error {
	ports := map[string]int{
		meta.ComponentNodeExporter:     options.NodeExporterPort,
		meta.ComponentBlackboxExporter: options.BlackboxExporterPort,
	}
	e := getter.Get(instance.GetHost())
	for _, comp := range []string{meta.ComponentNodeExporter, meta.ComponentBlackboxExporter} {
		log.Infof("Stopping component %s", comp)

		c := module.SystemdModuleConfig{
			Unit:   fmt.Sprintf("%s-%d.service", comp, ports[comp]),
			Action: "stop",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			log.Output(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to stop: %s", instance.GetHost())
		}

		if err := meta.PortStopped(e, ports[comp]); err != nil {
			str := fmt.Sprintf("\t%s failed to stop: %s", instance.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}
	}

	return nil
}

// StopComponent stop the instances.
func StopComponent(getter ExecutorGetter, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Stopping component %s", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		log.Infof("\tStopping instance %s", ins.GetHost())

		// Stop by systemd.
		c := module.SystemdModuleConfig{
			Unit:   ins.ServiceName(),
			Action: "stop",
			// Scope: "",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			log.Output(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to stop: %s", ins.GetHost())
		}

		err = ins.WaitForDown(e)
		if err != nil {
			str := fmt.Sprintf("\t%s failed to stop: %s", ins.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log.Infof("\tStop %s success", ins.GetHost())
	}

	return nil
}

/*
[tidb@ip-172-16-5-70 deploy]$ sudo systemctl status drainer-8249.service
● drainer-8249.service - drainer-8249 service
   Loaded: loaded (/etc/systemd/system/drainer-8249.service; disabled; vendor preset: disabled)
   Active: active (running) since Mon 2020-03-09 13:56:19 CST; 1 weeks 3 days ago
 Main PID: 36718 (drainer)
   CGroup: /system.slice/drainer-8249.service
           └─36718 bin/drainer --addr=172.16.5.70:8249 --pd-urls=http://172.16.5.70:2379 --data-dir=/data1/deploy/data.drainer --log-file=/data1/deploy/log/drainer.log --config=conf/drainer.toml --initial-commit-ts=408375872006389761

Mar 09 13:56:19 ip-172-16-5-70 systemd[1]: Started drainer-8249 service.
*/
// getServiceStatus return the Acitive line of status.
func getServiceStatus(e executor.TiOpsExecutor, name string) (active string, err error) {
	c := module.SystemdModuleConfig{
		Unit:   name,
		Action: "status",
	}
	systemd := module.NewSystemdModule(c)
	stdout, _, err := systemd.Execute(e)

	lines := strings.Split(string(stdout), "\n")
	if len(lines) >= 3 {
		return lines[2], nil
	}

	if err != nil {
		return
	}

	return "", errors.Errorf("unexpected output: %s", string(stdout))
}

// PrintClusterStatus print cluster status into the io.Writer.
func PrintClusterStatus(getter ExecutorGetter, spec *meta.Specification) (health bool) {
	health = true

	for _, com := range spec.ComponentsByStartOrder() {
		if len(com.Instances()) == 0 {
			continue
		}

		log.Infof(com.Name())
		for _, ins := range com.Instances() {
			log.Infof("\t%s", ins.GetHost())
			e := getter.Get(ins.GetHost())
			active, err := getServiceStatus(e, ins.ServiceName())
			if err != nil {
				health = false
				log.Errorf("\t\t%v", err)
			} else {
				log.Infof("\t\t%s", active)
			}
		}

	}

	return
}
