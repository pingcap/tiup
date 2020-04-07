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
	"strconv"
	"strings"
	"time"

	"github.com/pingcap-incubator/tiup-cluster/pkg/api"
	"github.com/pingcap-incubator/tiup-cluster/pkg/executor"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/module"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
	"golang.org/x/sync/errgroup"
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
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := spec.ComponentsByStopOrder()
	components = filterComponent(components, roleFilter)

	instCount := map[string]int{}
	spec.IterInstance(func(inst meta.Instance) {
		instCount[inst.GetHost()] = instCount[inst.GetHost()] + 1
	})

	for _, com := range components {
		insts := filterInstance(com.Instances(), nodeFilter)
		err := StopComponent(getter, insts)
		if err != nil {
			return errors.Annotatef(err, "failed to stop %s", com.Name())
		}
		for _, inst := range insts {
			instCount[inst.GetHost()]--
			if instCount[inst.GetHost()] == 0 {
				if err := StopMonitored(getter, inst, spec.MonitoredOptions); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// NeedCheckTomebsome return true if we need to check and destroy some node.
func NeedCheckTomebsome(spec *meta.Specification) bool {
	for _, s := range spec.TiKVServers {
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
	spec *meta.Specification,
	returNodesOnly bool,
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

		instances := (&meta.TiKVComponent{Specification: spec}).Instances()
		instances = filterID(instances, id)

		err = StopComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances)
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

		instances := (&meta.PumpComponent{Specification: spec}).Instances()
		instances = filterID(instances, id)
		err = StopComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances)
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

		instances := (&meta.DrainerComponent{Specification: spec}).Instances()
		instances = filterID(instances, id)

		err = StopComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}

		err = DestroyComponent(getter, instances)
		if err != nil {
			return nil, errors.AddStack(err)
		}
	}

	if returNodesOnly {
		return
	}

	spec.TiKVServers = kvServers
	spec.PumpServers = pumpServers
	spec.Drainers = drainerServers

	return
}

// Restart the cluster.
func Restart(
	getter ExecutorGetter,
	spec *meta.Specification,
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
			fmt.Println(string(stdout))
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

// RestartComponent restarts the component.
func RestartComponent(getter ExecutorGetter, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Restarting component %s", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		log.Infof("\tRestarting instance %s", ins.GetHost())

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
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to restart: %s", ins.GetHost())
		}

		// Check ready.
		err = ins.Ready(e)
		if err != nil {
			str := fmt.Sprintf("\t%s failed to restart: %s", ins.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log.Infof("\tRestart %s success", ins.GetHost())
	}

	return nil
}

func startInstance(getter ExecutorGetter, ins meta.Instance) error {
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
		Enabled:      true,
	}
	systemd := module.NewSystemdModule(c)
	stdout, stderr, err := systemd.Execute(e)

	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}
	if len(stderr) > 0 && !bytes.Contains(stderr, []byte("Created symlink ")) {
		log.Errorf(string(stderr))
	}

	if err != nil {
		return errors.Annotatef(err, "failed to start: %s %s:%d",
			ins.ComponentName(),
			ins.GetHost(),
			ins.GetPort())
	}

	// Check ready.
	err = ins.Ready(e)
	if err != nil {
		str := fmt.Sprintf("\t%s %s:%d failed to start: %s",
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

// StartComponent start the instances.
func StartComponent(getter ExecutorGetter, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Starting component %s", name)

	errg, _ := errgroup.WithContext(context.Background())

	for _, ins := range instances {
		ins := ins

		errg.Go(func() error {
			err := startInstance(getter, ins)
			if err != nil {
				return errors.AddStack(err)
			}
			return nil
		})
	}

	return errg.Wait()
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

		if err := meta.PortStopped(e, ports[comp]); err != nil {
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

func stopInstance(getter ExecutorGetter, ins meta.Instance) error {
	e := getter.Get(ins.GetHost())
	log.Infof("\tStopping instance %s", ins.GetHost())

	// Stop by systemd.
	c := module.SystemdModuleConfig{
		Unit:         ins.ServiceName(),
		Action:       "stop",
		ReloadDaemon: true, // always reload before operate
		// Scope: "",
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

	err = ins.WaitForDown(e)
	if err != nil {
		str := fmt.Sprintf("\t%s %s:%d failed to stop: %s",
			ins.ComponentName(),
			ins.GetHost(),
			ins.GetPort(),
			err)
		log.Errorf(str)
		return errors.Annotatef(err, str)
	}

	log.Infof("\tStop %s %s:%d success",
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
	log.Infof("Stopping component %s", name)

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

// GetServiceStatus return the Acitive line of status.
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
func GetServiceStatus(e executor.TiOpsExecutor, name string) (active string, err error) {
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
