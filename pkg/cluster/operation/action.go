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
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	"golang.org/x/sync/errgroup"
)

// Enable will enable/disable the cluster
func Enable(
	getter ExecutorGetter,
	cluster spec.Topology,
	options Options,
	isEnable bool,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := cluster.ComponentsByStartOrder()
	components = FilterComponent(components, roleFilter)
	monitoredOptions := cluster.GetMonitoredOptions()

	instCount := map[string]int{}
	cluster.IterInstance(func(inst spec.Instance) {
		instCount[inst.GetHost()]++
	})

	for _, comp := range components {
		insts := FilterInstance(comp.Instances(), nodeFilter)
		err := EnableComponent(getter, insts, options, isEnable)
		if err != nil {
			return errors.Annotatef(err, "failed to enable/disable %s", comp.Name())
		}
		if monitoredOptions == nil {
			continue
		}
		for _, inst := range insts {
			instCount[inst.GetHost()]--
			if instCount[inst.GetHost()] == 0 {
				if err := EnableMonitored(getter, inst, monitoredOptions, options.OptTimeout, isEnable); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Start the cluster.
func Start(
	getter ExecutorGetter,
	cluster spec.Topology,
	options Options,
	tlsCfg *tls.Config,
) error {
	uniqueHosts := set.NewStringSet()
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := cluster.ComponentsByStartOrder()
	components = FilterComponent(components, roleFilter)
	monitoredOptions := cluster.GetMonitoredOptions()

	for _, comp := range components {
		insts := FilterInstance(comp.Instances(), nodeFilter)
		err := StartComponent(getter, insts, options, tlsCfg)
		if err != nil {
			return errors.Annotatef(err, "failed to start %s", comp.Name())
		}
		if monitoredOptions == nil {
			continue
		}
		for _, inst := range insts {
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				if err := StartMonitored(getter, inst, monitoredOptions, options.OptTimeout); err != nil {
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
	cluster spec.Topology,
	options Options,
	tlsCfg *tls.Config,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := cluster.ComponentsByStopOrder()
	components = FilterComponent(components, roleFilter)

	instCount := map[string]int{}
	cluster.IterInstance(func(inst spec.Instance) {
		instCount[inst.GetHost()]++
	})

	for _, comp := range components {
		insts := FilterInstance(comp.Instances(), nodeFilter)
		err := StopComponent(getter, insts, options.OptTimeout)
		if err != nil && !options.Force {
			return errors.Annotatef(err, "failed to stop %s", comp.Name())
		}
		if cluster.GetMonitoredOptions() == nil {
			continue
		}
		for _, inst := range insts {
			instCount[inst.GetHost()]--
			if instCount[inst.GetHost()] == 0 {
				if err := StopMonitored(getter, inst, cluster.GetMonitoredOptions(), options.OptTimeout); err != nil && !options.Force {
					return err
				}
			}
		}
	}
	return nil
}

// NeedCheckTombstone return true if we need to check and destroy some node.
func NeedCheckTombstone(topo *spec.Specification) bool {
	for _, s := range topo.TiKVServers {
		if s.Offline {
			return true
		}
	}
	for _, s := range topo.TiFlashServers {
		if s.Offline {
			return true
		}
	}
	for _, s := range topo.PumpServers {
		if s.Offline {
			return true
		}
	}
	for _, s := range topo.Drainers {
		if s.Offline {
			return true
		}
	}

	return false
}

// Restart the cluster.
func Restart(
	getter ExecutorGetter,
	cluster spec.Topology,
	options Options,
	tlsCfg *tls.Config,
) error {
	err := Stop(getter, cluster, options, tlsCfg)
	if err != nil {
		return errors.Annotatef(err, "failed to stop")
	}

	err = Start(getter, cluster, options, tlsCfg)
	if err != nil {
		return errors.Annotatef(err, "failed to start")
	}

	return nil
}

// StartMonitored start BlackboxExporter and NodeExporter
func StartMonitored(getter ExecutorGetter, instance spec.Instance, options *spec.MonitoredOptions, timeout uint64) error {
	ports := map[string]int{
		spec.ComponentNodeExporter:     options.NodeExporterPort,
		spec.ComponentBlackboxExporter: options.BlackboxExporterPort,
	}
	e := getter.Get(instance.GetHost())
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		log.Infof("Starting component %s", comp)
		log.Infof("\tStarting instance %s", instance.GetHost())
		c := module.SystemdModuleConfig{
			Unit:         fmt.Sprintf("%s-%d.service", comp, ports[comp]),
			ReloadDaemon: true,
			Action:       "start",
			Timeout:      time.Second * time.Duration(timeout),
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to start: %s", instance.GetHost())
		}

		// Check ready.
		if err := spec.PortStarted(e, ports[comp], timeout); err != nil {
			return toFailedActionError(err, "start", instance)
		}

		log.Infof("\tStart %s success", instance.GetHost())
	}

	return nil
}

func restartInstance(getter ExecutorGetter, ins spec.Instance, timeout uint64) error {
	e := getter.Get(ins.GetHost())
	log.Infof("\tRestarting instance %s", ins.GetHost())

	// Restart by systemd.
	c := module.SystemdModuleConfig{
		Unit:         ins.ServiceName(),
		ReloadDaemon: true,
		Action:       "restart",
		Timeout:      time.Second * time.Duration(timeout),
	}
	systemd := module.NewSystemdModule(c)
	stdout, stderr, err := systemd.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 {
		log.Errorf(string(stderr))
	}

	if err != nil {
		return errors.Annotatef(err, "failed to restart: %s", ins.GetHost())
	}

	// Check ready.
	if err := ins.Ready(e, timeout); err != nil {
		return toFailedActionError(err, "restart", ins)
	}

	log.Infof("\tRestart %s success", ins.GetHost())

	return nil
}

// RestartComponent restarts the component.
func RestartComponent(getter ExecutorGetter, instances []spec.Instance, timeout uint64) error {
	if len(instances) == 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Restarting component %s", name)

	for _, ins := range instances {
		err := restartInstance(getter, ins, timeout)
		if err != nil {
			return err
		}
	}

	return nil
}

func enableInstance(getter ExecutorGetter, ins spec.Instance, timeout uint64, isEnable bool) error {
	e := getter.Get(ins.GetHost())
	if isEnable {
		log.Infof("\tEnabling instance %s %s:%d", ins.ComponentName(), ins.GetHost(), ins.GetPort())
	} else {
		log.Infof("\tDisabling instance %s %s:%d", ins.ComponentName(), ins.GetHost(), ins.GetPort())
	}

	action := "disable"
	if isEnable {
		action = "enable"
	}

	// Enable/Disable by systemd.
	c := module.SystemdModuleConfig{
		Unit:    ins.ServiceName(),
		Action:  action,
		Timeout: time.Second * time.Duration(timeout),
	}
	systemd := module.NewSystemdModule(c)
	stdout, stderr, err := systemd.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 && !bytes.Contains(stderr, []byte("Created symlink ")) && !bytes.Contains(stderr, []byte("Removed symlink ")) {
		log.Errorf(string(stderr))
	}

	if err != nil {
		return errors.Annotatef(err, "failed to %s: %s %s:%d",
			action,
			ins.ComponentName(),
			ins.GetHost(),
			ins.GetPort())
	}

	if isEnable {
		log.Infof("\tEnable %s %s:%d success", ins.ComponentName(), ins.GetHost(), ins.GetPort())
	} else {
		log.Infof("\tDisable %s %s:%d success", ins.ComponentName(), ins.GetHost(), ins.GetPort())
	}

	return nil
}

func startInstance(getter ExecutorGetter, ins spec.Instance, timeout uint64) error {
	e := getter.Get(ins.GetHost())
	log.Infof("\tStarting instance %s %s:%d",
		ins.ComponentName(),
		ins.GetHost(),
		ins.GetPort())

	// Start by systemd.
	c := module.SystemdModuleConfig{
		Unit:         ins.ServiceName(),
		ReloadDaemon: true,
		Action:       "start",
		Timeout:      time.Second * time.Duration(timeout),
	}
	systemd := module.NewSystemdModule(c)
	stdout, stderr, err := systemd.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 && !bytes.Contains(stderr, []byte("Created symlink ")) && !bytes.Contains(stderr, []byte("Removed symlink ")) {
		log.Errorf(string(stderr))
	}

	if err != nil {
		return toFailedActionError(err, "start", ins)
	}

	// Check ready.
	if err := ins.Ready(e, timeout); err != nil {
		return toFailedActionError(err, "start", ins)
	}

	log.Infof("\tStart %s %s:%d success",
		ins.ComponentName(),
		ins.GetHost(),
		ins.GetPort())

	return nil
}

// EnableComponent enable/disable the instances
func EnableComponent(getter ExecutorGetter, instances []spec.Instance, options Options, isEnable bool) error {
	if len(instances) == 0 {
		return nil
	}

	name := instances[0].ComponentName()
	if isEnable {
		log.Infof("Enabling component %s", name)
	} else {
		log.Infof("Disabling component %s", name)
	}

	errg, _ := errgroup.WithContext(context.Background())

	for _, ins := range instances {
		ins := ins

		errg.Go(func() error {
			err := enableInstance(getter, ins, options.OptTimeout, isEnable)
			if err != nil {
				return err
			}
			return nil
		})
	}

	return errg.Wait()
}

// EnableMonitored enable/disable monitor service in a cluster
func EnableMonitored(
	getter ExecutorGetter, instance spec.Instance,
	options *spec.MonitoredOptions, timeout uint64, isEnable bool,
) error {
	action := "disable"
	if isEnable {
		action = "enable"
	}

	ports := map[string]int{
		spec.ComponentNodeExporter:     options.NodeExporterPort,
		spec.ComponentBlackboxExporter: options.BlackboxExporterPort,
	}
	e := getter.Get(instance.GetHost())
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		if isEnable {
			log.Infof("Enabling component %s", comp)
		} else {
			log.Infof("Disabling component %s", comp)
		}

		c := module.SystemdModuleConfig{
			Unit:    fmt.Sprintf("%s-%d.service", comp, ports[comp]),
			Action:  action,
			Timeout: time.Second * time.Duration(timeout),
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 && !bytes.Contains(stderr, []byte("Created symlink ")) && !bytes.Contains(stderr, []byte("Removed symlink ")) {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return toFailedActionError(err, action, instance)
		}
	}

	return nil
}

// StartComponent start the instances.
func StartComponent(getter ExecutorGetter, instances []spec.Instance, options Options, tlsCfg *tls.Config) error {
	if len(instances) == 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Starting component %s", name)

	errg, _ := errgroup.WithContext(context.Background())

	for _, ins := range instances {
		ins := ins

		errg.Go(func() error {
			if err := ins.PrepareStart(tlsCfg); err != nil {
				return err
			}
			err := startInstance(getter, ins, options.OptTimeout)
			if err != nil {
				return err
			}
			return nil
		})
	}

	return errg.Wait()
}

// StopMonitored stop BlackboxExporter and NodeExporter
func StopMonitored(getter ExecutorGetter, instance spec.Instance, options *spec.MonitoredOptions, timeout uint64) error {
	ports := map[string]int{
		spec.ComponentNodeExporter:     options.NodeExporterPort,
		spec.ComponentBlackboxExporter: options.BlackboxExporterPort,
	}
	e := getter.Get(instance.GetHost())
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		log.Infof("Stopping component %s", comp)

		c := module.SystemdModuleConfig{
			Unit:         fmt.Sprintf("%s-%d.service", comp, ports[comp]),
			Action:       "stop",
			ReloadDaemon: true,
			Timeout:      time.Second * time.Duration(timeout),
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}

		if len(stderr) > 0 {
			// ignore "unit not loaded" error, as this means the unit is not
			// exist, and that's exactly what we want
			// NOTE: there will be a potential bug if the unit name is set
			// wrong and the real unit still remains started.
			if bytes.Contains(stderr, []byte(" not loaded.")) {
				log.Warnf(string(stderr))
				err = nil // reset the error to avoid exiting
			} else {
				log.Errorf(string(stderr))
			}
		}

		if err != nil {
			return toFailedActionError(err, "stop", instance)
		}

		if err := spec.PortStopped(e, ports[comp], timeout); err != nil {
			return toFailedActionError(err, "stop", instance)
		}
	}

	return nil
}

func stopInstance(getter ExecutorGetter, ins spec.Instance, timeout uint64) error {
	e := getter.Get(ins.GetHost())
	log.Infof("\tStopping instance %s", ins.GetHost())

	// Stop by systemd.
	c := module.SystemdModuleConfig{
		Unit:         ins.ServiceName(),
		Action:       "stop",
		ReloadDaemon: true, // always reload before operate
		Timeout:      time.Second * time.Duration(timeout),
	}
	systemd := module.NewSystemdModule(c)
	stdout, stderr, err := systemd.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 {
		// ignore "unit not loaded" error, as this means the unit is not
		// exist, and that's exactly what we want
		// NOTE: there will be a potential bug if the unit name is set
		// wrong and the real unit still remains started.
		if bytes.Contains(stderr, []byte(" not loaded.")) {
			log.Warnf(string(stderr))
			err = nil // reset the error to avoid exiting
		} else {
			log.Errorf(string(stderr))
		}
	}

	if err != nil {
		return toFailedActionError(err, "stop", ins)
	}

	log.Infof("\tStop %s %s:%d success",
		ins.ComponentName(),
		ins.GetHost(),
		ins.GetPort())

	return nil
}

// StopComponent stop the instances.
func StopComponent(getter ExecutorGetter, instances []spec.Instance, timeout uint64) error {
	if len(instances) == 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Stopping component %s", name)

	errg, _ := errgroup.WithContext(context.Background())

	for _, ins := range instances {
		ins := ins
		errg.Go(func() error {
			err := stopInstance(getter, ins, timeout)
			if err != nil {
				return err
			}
			return nil
		})
	}

	return errg.Wait()
}

// PrintClusterStatus print cluster status into the io.Writer.
func PrintClusterStatus(getter ExecutorGetter, cluster *spec.Specification) (health bool) {
	health = true

	for _, com := range cluster.ComponentsByStartOrder() {
		if len(com.Instances()) == 0 {
			continue
		}

		log.Infof("Checking service state of %s", com.Name())
		errg, _ := errgroup.WithContext(context.Background())
		for _, ins := range com.Instances() {
			ins := ins

			errg.Go(func() error {
				e := getter.Get(ins.GetHost())
				active, err := GetServiceStatus(e, ins.ServiceName())
				if err != nil {
					health = false
					log.Errorf("\t%s\t%v", ins.GetHost(), err)
				} else {
					log.Infof("\t%s\t%s", ins.GetHost(), active)
				}
				return nil
			})
		}
		_ = errg.Wait()
	}

	return
}

// toFailedActionError formats the errror msg for failed action
func toFailedActionError(err error, action string, inst spec.Instance) error {
	return errors.Annotatef(err,
		"failed to %s: %s %s:%d, please check the instance's log(%s) for more detail.",
		action,
		inst.ComponentName(),
		inst.GetHost(),
		inst.GetPort(),
		inst.LogDir(),
	)
}
