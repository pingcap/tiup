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
	"strconv"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
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
	uniqueHosts := set.NewStringSet()
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := cluster.ComponentsByStartOrder()
	components = FilterComponent(components, roleFilter)

	for _, com := range components {
		insts := FilterInstance(com.Instances(), nodeFilter)
		err := EnableComponent(getter, insts, options, isEnable)
		if err != nil {
			return errors.Annotatef(err, "failed to start %s", com.Name())
		}
		for _, inst := range insts {
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				if cluster.GetMonitoredOptions() != nil {
					if err := EnableMonitored(getter, inst, cluster.GetMonitoredOptions(), options.OptTimeout, isEnable); err != nil {
						return err
					}
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

	for _, com := range components {
		insts := FilterInstance(com.Instances(), nodeFilter)
		err := StartComponent(getter, insts, options, tlsCfg)
		if err != nil {
			return errors.Annotatef(err, "failed to start %s", com.Name())
		}
		for _, inst := range insts {
			if !uniqueHosts.Exist(inst.GetHost()) {
				uniqueHosts.Insert(inst.GetHost())
				if cluster.GetMonitoredOptions() != nil {
					if err := StartMonitored(getter, inst, cluster.GetMonitoredOptions(), options.OptTimeout); err != nil {
						return err
					}
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
		instCount[inst.GetHost()] = instCount[inst.GetHost()] + 1
	})

	for _, com := range components {
		insts := FilterInstance(com.Instances(), nodeFilter)
		err := StopComponent(getter, insts, options.OptTimeout)
		if err != nil && !options.Force {
			return errors.Annotatef(err, "failed to stop %s", com.Name())
		}
		for _, inst := range insts {
			instCount[inst.GetHost()]--
			if instCount[inst.GetHost()] == 0 {
				if cluster.GetMonitoredOptions() != nil {
					if err := StopMonitored(getter, inst, cluster.GetMonitoredOptions(), options.OptTimeout); err != nil && !options.Force {
						return err
					}
				}
			}
		}
	}
	return nil
}

// NeedCheckTomebsome return true if we need to check and destroy some node.
func NeedCheckTomebsome(spec *spec.Specification) bool {
	for _, s := range spec.TiKVServers {
		if s.Offline {
			return true
		}
	}
	for _, s := range spec.TiFlashServers {
		if s.Offline {
			return true
		}
	}
	for _, s := range spec.PumpServers {
		if s.Offline {
			return true
		}
	}
	for _, s := range spec.Drainers {
		if s.Offline {
			return true
		}
	}
	return false
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
	var pdClient = api.NewPDClient(cluster.GetPDList(), 10*time.Second, tlsCfg)

	binlogClient, err := api.NewBinlogClient(cluster.GetPDList(), tlsCfg)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	filterID := func(instance []spec.Instance, id string) (res []spec.Instance) {
		for _, ins := range instance {
			if ins.ID() == id {
				res = append(res, ins)
			}
		}
		return
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
			return nil, errors.AddStack(err)
		}

		if !tombstone {
			kvServers = append(kvServers, s)
			continue
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.TiKVComponent{Specification: cluster}).Instances()
		instances = filterID(instances, id)

		err = StopComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, cluster, options)
		if err != nil {
			return nil, errors.AddStack(err)
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
			return nil, errors.AddStack(err)
		}

		if !tombstone {
			flashServers = append(flashServers, s)
			continue
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.TiFlashComponent{Specification: cluster}).Instances()
		id = s.Host + ":" + strconv.Itoa(s.GetMainPort())
		instances = filterID(instances, id)

		err = StopComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, cluster, options)
		if err != nil {
			return nil, errors.AddStack(err)
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
			return nil, errors.AddStack(err)
		}

		if !tombstone {
			pumpServers = append(pumpServers, s)
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.PumpComponent{Specification: cluster}).Instances()
		instances = filterID(instances, id)
		err = StopComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, cluster, options)
		if err != nil {
			return nil, errors.AddStack(err)
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
			return nil, errors.AddStack(err)
		}

		if !tombstone {
			drainerServers = append(drainerServers, s)
		}

		nodes = append(nodes, id)
		if returNodesOnly {
			continue
		}

		instances := (&spec.DrainerComponent{Specification: cluster}).Instances()
		instances = filterID(instances, id)

		err = StopComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, cluster, options)
		if err != nil {
			return nil, errors.AddStack(err)
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
func StartMonitored(getter ExecutorGetter, instance spec.Instance, options *spec.MonitoredOptions, timeout int64) error {
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
			str := fmt.Sprintf("\t%s failed to start: %s", instance.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log.Infof("\tStart %s success", instance.GetHost())
	}

	return nil
}

func restartInstance(getter ExecutorGetter, ins spec.Instance, timeout int64) error {
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
	err = ins.Ready(e, timeout)
	if err != nil {
		str := fmt.Sprintf("\t%s failed to restart: %s", ins.GetHost(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}

	log.Infof("\tRestart %s success", ins.GetHost())

	return nil
}

// RestartComponent restarts the component.
func RestartComponent(getter ExecutorGetter, instances []spec.Instance, timeout int64) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Restarting component %s", name)

	for _, ins := range instances {
		err := restartInstance(getter, ins, timeout)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	return nil
}

func enableInstance(getter ExecutorGetter, ins spec.Instance, timeout int64, isEnable bool) error {
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

func startInstance(getter ExecutorGetter, ins spec.Instance, timeout int64) error {
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
		return errors.Annotatef(err, "failed to start: %s %s:%d",
			ins.ComponentName(),
			ins.GetHost(),
			ins.GetPort())
	}

	// Check ready.
	err = ins.Ready(e, timeout)
	if err != nil {
		str := fmt.Sprintf("\t%s %s:%d failed to start: %s, please check the log of the instance",
			ins.ComponentName(),
			ins.GetHost(),
			ins.GetPort(), err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
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
				return errors.AddStack(err)
			}
			return nil
		})
	}

	return errg.Wait()
}

// EnableMonitored enable/disable monitor service in a cluster
func EnableMonitored(
	getter ExecutorGetter, instance spec.Instance,
	options *spec.MonitoredOptions, timeout int64, isEnable bool,
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
			return errors.Annotatef(err, "failed to %s: %s %s:%d",
				action,
				instance.ComponentName(),
				instance.GetHost(),
				instance.GetPort())
		}
	}

	return nil
}

// StartComponent start the instances.
func StartComponent(getter ExecutorGetter, instances []spec.Instance, options Options, tlsCfg *tls.Config) error {
	if len(instances) <= 0 {
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
				return errors.AddStack(err)
			}
			return nil
		})
	}

	return errg.Wait()
}

// StopMonitored stop BlackboxExporter and NodeExporter
func StopMonitored(getter ExecutorGetter, instance spec.Instance, options *spec.MonitoredOptions, timeout int64) error {
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
			return errors.Annotatef(err, "failed to stop: %s %s:%d",
				instance.ComponentName(),
				instance.GetHost(),
				instance.GetPort())
		}

		if err := spec.PortStopped(e, ports[comp], timeout); err != nil {
			str := fmt.Sprintf("\t%s %s:%d failed to stop: %s",
				instance.ComponentName(),
				instance.GetHost(),
				instance.GetPort(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}
	}

	return nil
}

func stopInstance(getter ExecutorGetter, ins spec.Instance, timeout int64) error {
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
		return errors.Annotatef(err, "failed to stop: %s %s:%d",
			ins.ComponentName(),
			ins.GetHost(),
			ins.GetPort())
	}

	log.Infof("\tStop %s %s:%d success",
		ins.ComponentName(),
		ins.GetHost(),
		ins.GetPort())

	return nil
}

// StopComponent stop the instances.
func StopComponent(getter ExecutorGetter, instances []spec.Instance, timeout int64) error {
	if len(instances) <= 0 {
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
				return errors.AddStack(err)
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
