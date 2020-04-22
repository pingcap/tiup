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
	"strconv"
	"time"

	"github.com/pingcap-incubator/tiup-cluster/pkg/api"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
)

// Upgrade the cluster.
func Upgrade(
	getter ExecutorGetter,
	spec meta.Specification,
	options Options,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := spec.ComponentsByStartOrder()
	components = FilterComponent(components, roleFilter)

	leaderAware := set.NewStringSet(meta.ComponentPD, meta.ComponentTiKV)

	timeoutOpt := &utils.RetryOption{
		Timeout: time.Second * time.Duration(options.Timeout),
		Delay:   time.Second * 2,
	}

	for _, component := range components {
		instances := FilterInstance(component.Instances(), nodeFilter)
		if len(instances) < 1 {
			continue
		}

		if clusterSpec := spec.GetClusterSpecification(); clusterSpec != nil {
			// Transfer leader of evict leader if the component is TiKV/PD in non-force mode
			if !options.Force && leaderAware.Exist(component.Name()) {
				pdClient := api.NewPDClient(clusterSpec.GetPDList(), 5*time.Second, nil)
				switch component.Name() {
				case meta.ComponentPD:
					log.Infof("Restarting component %s", component.Name())

					for _, instance := range instances {
						leader, err := pdClient.GetLeader()
						if err != nil {
							return errors.Annotatef(err, "failed to get PD leader %s", instance.GetHost())
						}

						if len(clusterSpec.PDServers) > 1 && leader.Name == instance.(*meta.PDInstance).Name {
							if err := pdClient.EvictPDLeader(timeoutOpt); err != nil {
								return errors.Annotatef(err, "failed to evict PD leader %s", instance.GetHost())
							}
						}

						if err := stopInstance(getter, instance); err != nil {
							return errors.Annotatef(err, "failed to stop %s", instance.GetHost())
						}
						if err := startInstance(getter, instance); err != nil {
							return errors.Annotatef(err, "failed to start %s", instance.GetHost())
						}
					}

				case meta.ComponentTiKV:
					log.Infof("Restarting component %s", component.Name())
					pdClient := api.NewPDClient(clusterSpec.GetPDList(), 5*time.Second, nil)
					// Make sure there's leader of PD.
					// Although we evict pd leader when restart pd,
					// But when there's only one PD instance the pd might not serve request right away after restart.
					err := pdClient.WaitLeader(timeoutOpt)
					if err != nil {
						return errors.Annotate(err, "failed to wait leader")
					}

					for _, instance := range instances {
						if err := pdClient.EvictStoreLeader(addr(instance), timeoutOpt); err != nil {
							if utils.IsTimeoutOrMaxRetry(err) {
								log.Warnf("Ignore evicting store leader from %s, %v", instance.ID(), err)
							} else {
								return errors.Annotatef(err, "failed to evict store leader %s", instance.GetHost())
							}
						}

						if err := stopInstance(getter, instance); err != nil {
							return errors.Annotatef(err, "failed to stop %s", instance.GetHost())
						}
						if err := startInstance(getter, instance); err != nil {
							return errors.Annotatef(err, "failed to start %s", instance.GetHost())
						}
						// remove store leader evict scheduler after restart
						if err := pdClient.RemoveStoreEvict(addr(instance)); err != nil {
							return errors.Annotatef(err, "failed to remove evict store scheduler for %s", instance.GetHost())
						}
					}
				}
				continue
			}
		}

		if err := RestartComponent(getter, instances); err != nil {
			return errors.Annotatef(err, "failed to restart %s", component.Name())
		}
	}

	return nil
}

func addr(ins meta.Instance) string {
	if ins.GetPort() == 0 || ins.GetPort() == 80 {
		panic(ins)
	}
	return ins.GetHost() + ":" + strconv.Itoa(ins.GetPort())
}
