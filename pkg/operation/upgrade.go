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
	"io"
	"strconv"
	"time"

	"github.com/pingcap-incubator/tiops/pkg/api"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
)

// Upgrade the cluster.
func Upgrade(
	getter ExecutorGetter,
	w io.Writer,
	spec *meta.Specification,
	options Options,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := spec.ComponentsByStartOrder()
	components = filterComponent(components, roleFilter)

	leaderAware := set.NewStringSet(meta.ComponentPD, meta.ComponentTiKV)

	var pdAddrs []string

	for _, component := range components {
		if component.Name() == meta.ComponentPD {
			for _, instance := range component.Instances() {
				pdAddrs = append(pdAddrs, addr(instance))
			}
		}

		instances := filterInstance(component.Instances(), nodeFilter)
		if len(instances) < 1 {
			continue
		}

		// Transfer leader of evict leader if the component is TiKV/PD in non-force mode
		if !options.Force && leaderAware.Exist(component.Name()) {
			switch component.Name() {
			case meta.ComponentPD:
				for _, instance := range instances {
					pdClient := api.NewPDClient(addr(instance), 5*time.Second, nil)
					leader, err := pdClient.GetLeader()
					if err != nil {
						return errors.Annotatef(err, "failed to get PD leader %s", instance.GetHost())
					}
					if leader.Name == instance.(*meta.PDInstance).Name {
						if err := pdClient.EvictPDLeader(); err != nil {
							return errors.Annotatef(err, "failed to evict PD leader %s", instance.GetHost())
						}
					}
					if err := StopComponent(getter, w, []meta.Instance{instance}); err != nil {
						return errors.Annotatef(err, "failed to stop %s", component.Name())
					}
					if err := StartComponent(getter, w, []meta.Instance{instance}); err != nil {
						return errors.Annotatef(err, "failed to start %s", component.Name())
					}
				}

			case meta.ComponentTiKV:
				if pdAddrs == nil || len(pdAddrs) <= 0 {
					return errors.New("cannot find pd addr")
				}

				for _, instance := range instances {
					pdClient := api.NewPDClient(pdAddrs[0], 5*time.Second, nil)
					if err := pdClient.EvictStoreLeader(addr(instance)); err != nil {
						return errors.Annotatef(err, "failed to evict store leader %s", instance.GetHost())
					}
					if err := StopComponent(getter, w, []meta.Instance{instance}); err != nil {
						return errors.Annotatef(err, "failed to stop %s", component.Name())
					}
					if err := StartComponent(getter, w, []meta.Instance{instance}); err != nil {
						return errors.Annotatef(err, "failed to start %s", component.Name())
					}
				}
			}
			continue
		}

		if err := StopComponent(getter, w, instances); err != nil {
			return errors.Annotatef(err, "failed to stop %s", component.Name())
		}
		if err := StartComponent(getter, w, instances); err != nil {
			return errors.Annotatef(err, "failed to start %s", component.Name())
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
