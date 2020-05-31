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
	"fmt"
	log2 "github.com/pingcap-incubator/tiup/pkg/logger/log"
	"strconv"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/cluster/api"
	"github.com/pingcap-incubator/tiup/pkg/cluster/meta"
	"github.com/pingcap-incubator/tiup/pkg/cluster/module"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
	"golang.org/x/sync/errgroup"
)

// Start the cluster.
func Start(
	getter ExecutorGetter,
	spec meta.Specification,
	options Options,
) error {
	uniqueHosts := set.NewStringSet()
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := spec.ComponentsByStartOrder()
	components = FilterComponent(components, roleFilter)

	for _, com := range components {
		insts := FilterInstance(com.Instances(), nodeFilter)
		err := StartComponent(getter, insts, options)
		if err != nil {
			return errors.Annotatef(err, "failed to start %s", com.Name())
		}
		if clusterSpec := spec.GetClusterSpecification(); clusterSpec != nil {
			for _, inst := range insts {
				if !uniqueHosts.Exist(inst.GetHost()) {
					uniqueHosts.Insert(inst.GetHost())
					if err := StartMonitored(getter, inst, clusterSpec.MonitoredOptions, options.OptTimeout); err != nil {
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
	spec meta.Specification,
	options Options,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := spec.ComponentsByStopOrder()
	components = FilterComponent(components, roleFilter)

	instCount := map[string]int{}
	spec.IterInstance(func(inst meta.Instance) {
		instCount[inst.GetHost()] = instCount[inst.GetHost()] + 1
	})

	for _, com := range components {
		insts := FilterInstance(com.Instances(), nodeFilter)
		err := StopComponent(getter, insts)
		if err != nil {
			return errors.Annotatef(err, "failed to stop %s", com.Name())
		}
		if clusterSpec := spec.GetClusterSpecification(); clusterSpec != nil {
			for _, inst := range insts {
				instCount[inst.GetHost()]--
				if instCount[inst.GetHost()] == 0 {
					if err := StopMonitored(getter, inst, clusterSpec.MonitoredOptions, options.OptTimeout); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// NeedCheckTomebsome return true if we need to check and destroy some node.
func NeedCheckTomebsome(spec *meta.ClusterSpecification) bool {
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
	spec meta.Specification,
	returNodesOnly bool,
	options Options,
) (nodes []string, err error) {
	if clusterSpec := spec.GetClusterSpecification(); clusterSpec != nil {
		return DestroyClusterTombstone(getter, clusterSpec, returNodesOnly, options)
	}
	return nil, nil
}

// DestroyClusterTombstone remove the tombstone node in spec and destroy them.
// If returNodesOnly is true, it will only return the node id that can be destroy.
func DestroyClusterTombstone(
	getter ExecutorGetter,
	spec *meta.ClusterSpecification,
	returNodesOnly bool,
	options Options,
) (nodes []string, err error) {
	var pdClient = api.NewPDClient(spec.GetPDList(), 10*time.Second, nil)

	binlogClient, err := api.NewBinlogClient(spec.GetPDList(), nil)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	filterID := func(instance []meta.Instance, id string) (res []meta.Instance) {
		for _, ins := range instance {
			if ins.ID() == id {
				res = append(res, ins)
			}
		}
		return
	}

	var kvServers []meta.TiKVSpec
	for _, s := range spec.TiKVServers {
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

		instances := (&meta.TiKVComponent{ClusterSpecification: spec}).Instances()
		instances = filterID(instances, id)

		err = StopComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}

	}

	var flashServers []meta.TiFlashSpec
	for _, s := range spec.TiFlashServers {
		if !s.Offline {
			flashServers = append(flashServers, s)
			continue
		}

		id := s.Host + ":" + strconv.Itoa(s.FlashProxyPort)

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

		instances := (&meta.TiFlashComponent{ClusterSpecification: spec}).Instances()
		instances = filterID(instances, id)

		err = StopComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}

	}

	var pumpServers []meta.PumpSpec
	for _, s := range spec.PumpServers {
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

		instances := (&meta.PumpComponent{ClusterSpecification: spec}).Instances()
		instances = filterID(instances, id)
		err = StopComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}

	}

	var drainerServers []meta.DrainerSpec
	for _, s := range spec.Drainers {
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

		instances := (&meta.DrainerComponent{ClusterSpecification: spec}).Instances()
		instances = filterID(instances, id)

		err = StopComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances, options.OptTimeout)
		if err != nil {
			return nil, errors.AddStack(err)
		}
	}

	if returNodesOnly {
		return
	}

	spec.TiKVServers = kvServers
	spec.TiFlashServers = flashServers
	spec.PumpServers = pumpServers
	spec.Drainers = drainerServers

	return
}

// Restart the cluster.
func Restart(
	getter ExecutorGetter,
	spec meta.Specification,
	options Options,
) error {
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
func StartMonitored(getter ExecutorGetter, instance meta.Instance, options meta.MonitoredOptions, timeout int64) error {
	ports := map[string]int{
		meta.ComponentNodeExporter:     options.NodeExporterPort,
		meta.ComponentBlackboxExporter: options.BlackboxExporterPort,
	}
	e := getter.Get(instance.GetHost())
	for _, comp := range []string{meta.ComponentNodeExporter, meta.ComponentBlackboxExporter} {
		log2.Infof("Starting component %s", comp)
		log2.Infof("\tStarting instance %s", instance.GetHost())
		c := module.SystemdModuleConfig{
			Unit:         fmt.Sprintf("%s-%d.service", comp, ports[comp]),
			ReloadDaemon: true,
			Action:       "start",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			log2.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to start: %s", instance.GetHost())
		}

		// Check ready.
		if err := meta.PortStarted(e, ports[comp], timeout); err != nil {
			str := fmt.Sprintf("\t%s failed to start: %s", instance.GetHost(), err)
			log2.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log2.Infof("\tStart %s success", instance.GetHost())
	}

	return nil
}

// RestartComponent restarts the component.
func RestartComponent(getter ExecutorGetter, instances []meta.Instance, timeout int64) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log2.Infof("Restarting component %s", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		log2.Infof("\tRestarting instance %s", ins.GetHost())

		// Restart by systemd.
		c := module.SystemdModuleConfig{
			Unit:         ins.ServiceName(),
			ReloadDaemon: true,
			Action:       "restart",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		if len(stdout) > 0 {
			fmt.Println(string(stdout))
		}
		if len(stderr) > 0 {
			log2.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to restart: %s", ins.GetHost())
		}

		// Check ready.
		err = ins.Ready(e, timeout)
		if err != nil {
			str := fmt.Sprintf("\t%s failed to restart: %s", ins.GetHost(), err)
			log2.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log2.Infof("\tRestart %s success", ins.GetHost())
	}

	return nil
}

func startInstance(getter ExecutorGetter, ins meta.Instance, timeout int64) error {
	e := getter.Get(ins.GetHost())
	log2.Infof("\tStarting instance %s %s:%d",
		ins.ComponentName(),
		ins.GetHost(),
		ins.GetPort())

	// Start by systemd.
	c := module.SystemdModuleConfig{
		Unit:         ins.ServiceName(),
		ReloadDaemon: true,
		Action:       "start",
		Enabled:      true,
	}
	systemd := module.NewSystemdModule(c)
	stdout, stderr, err := systemd.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 && !bytes.Contains(stderr, []byte("Created symlink ")) {
		log2.Errorf(string(stderr))
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
		log2.Errorf(str)
		return errors.Annotatef(err, str)
	}

	log2.Infof("\tStart %s %s:%d success",
		ins.ComponentName(),
		ins.GetHost(),
		ins.GetPort())

	return nil
}

// StartComponent start the instances.
func StartComponent(getter ExecutorGetter, instances []meta.Instance, options Options) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log2.Infof("Starting component %s", name)

	errg, _ := errgroup.WithContext(context.Background())

	for _, ins := range instances {
		ins := ins

		errg.Go(func() error {
			if err := ins.PrepareStart(); err != nil {
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
func StopMonitored(getter ExecutorGetter, instance meta.Instance, options meta.MonitoredOptions, timeout int64) error {
	ports := map[string]int{
		meta.ComponentNodeExporter:     options.NodeExporterPort,
		meta.ComponentBlackboxExporter: options.BlackboxExporterPort,
	}
	e := getter.Get(instance.GetHost())
	for _, comp := range []string{meta.ComponentNodeExporter, meta.ComponentBlackboxExporter} {
		log2.Infof("Stopping component %s", comp)

		c := module.SystemdModuleConfig{
			Unit:         fmt.Sprintf("%s-%d.service", comp, ports[comp]),
			Action:       "stop",
			ReloadDaemon: true,
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
				log2.Warnf(string(stderr))
				err = nil // reset the error to avoid exiting
			} else {
				log2.Errorf(string(stderr))
			}
		}

		if err != nil {
			return errors.Annotatef(err, "failed to stop: %s %s:%d",
				instance.ComponentName(),
				instance.GetHost(),
				instance.GetPort())
		}

		if err := meta.PortStopped(e, ports[comp], timeout); err != nil {
			str := fmt.Sprintf("\t%s %s:%d failed to stop: %s",
				instance.ComponentName(),
				instance.GetHost(),
				instance.GetPort(), err)
			log2.Errorf(str)
			return errors.Annotatef(err, str)
		}
	}

	return nil
}

func stopInstance(getter ExecutorGetter, ins meta.Instance) error {
	e := getter.Get(ins.GetHost())
	log2.Infof("\tStopping instance %s", ins.GetHost())

	// Stop by systemd.
	c := module.SystemdModuleConfig{
		Unit:         ins.ServiceName(),
		Action:       "stop",
		ReloadDaemon: true, // always reload before operate
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
			log2.Warnf(string(stderr))
			err = nil // reset the error to avoid exiting
		} else {
			log2.Errorf(string(stderr))
		}
	}

	if err != nil {
		return errors.Annotatef(err, "failed to stop: %s %s:%d",
			ins.ComponentName(),
			ins.GetHost(),
			ins.GetPort())
	}

	log2.Infof("\tStop %s %s:%d success",
		ins.ComponentName(),
		ins.GetHost(),
		ins.GetPort())

	return nil
}

// StopComponent stop the instances.
func StopComponent(getter ExecutorGetter, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log2.Infof("Stopping component %s", name)

	errg, _ := errgroup.WithContext(context.Background())

	for _, ins := range instances {
		ins := ins
		errg.Go(func() error {

			err := stopInstance(getter, ins)
			if err != nil {
				return errors.AddStack(err)
			}
			return nil
		})
	}

	return errg.Wait()
}

// PrintClusterStatus print cluster status into the io.Writer.
func PrintClusterStatus(getter ExecutorGetter, spec meta.Specification) (health bool) {
	health = true

	for _, com := range spec.ComponentsByStartOrder() {
		if len(com.Instances()) == 0 {
			continue
		}

		log2.Infof("Checking service state of %s", com.Name())
		errg, _ := errgroup.WithContext(context.Background())
		for _, ins := range com.Instances() {
			ins := ins

			errg.Go(func() error {
				e := getter.Get(ins.GetHost())
				active, err := GetServiceStatus(e, ins.ServiceName())
				if err != nil {
					health = false
					log2.Errorf("\t%s\t%v", ins.GetHost(), err)
				} else {
					log2.Infof("\t%s\t%s", ins.GetHost(), active)
				}
				return nil
			})
		}
		_ = errg.Wait()
	}

	return
}
