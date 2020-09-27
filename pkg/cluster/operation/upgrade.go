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
	"crypto/tls"
	"strconv"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
)

// Upgrade the cluster.
func Upgrade(
	getter ExecutorGetter,
	topo spec.Topology,
	options Options,
	tlsCfg *tls.Config,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := topo.ComponentsByUpdateOrder()
	components = FilterComponent(components, roleFilter)

	for _, component := range components {
		instances := FilterInstance(component.Instances(), nodeFilter)
		if len(instances) < 1 {
			continue
		}

		// Transfer leader of evict leader if the component is TiKV/PD in non-force mode

		log.Infof("Restarting component %s", component.Name())

		for _, instance := range instances {
			var rollingInstance spec.RollingUpdateInstance
			var isRollingInstance bool

			if !options.Force {
				rollingInstance, isRollingInstance = instance.(spec.RollingUpdateInstance)
			}

			if isRollingInstance {
				err := rollingInstance.PreRestart(topo, int(options.APITimeout), tlsCfg)
				if err != nil && !options.Force {
					return errors.AddStack(err)
				}
			}

			if err := restartInstance(getter, instance, options.OptTimeout); err != nil && !options.Force {
				return errors.AddStack(err)
			}

			if isRollingInstance {
				err := rollingInstance.PostRestart(topo, tlsCfg)
				if err != nil && !options.Force {
					return errors.AddStack(err)
				}
			}
		}
	}

	return nil
}

// Addr returns the address of the instance.
func Addr(ins spec.Instance) string {
	if ins.GetPort() == 0 || ins.GetPort() == 80 {
		panic(ins)
	}
	return ins.GetHost() + ":" + strconv.Itoa(ins.GetPort())
}
