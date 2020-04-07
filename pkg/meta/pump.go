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

package meta

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/pingcap-incubator/tiup-cluster/pkg/executor"
	"github.com/pingcap-incubator/tiup-cluster/pkg/template/scripts"
)

// PumpComponent represents Pump component.
type PumpComponent struct{ *Specification }

// Name implements Component interface.
func (c *PumpComponent) Name() string {
	return ComponentPump
}

// Instances implements Component interface.
func (c *PumpComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.PumpServers))
	for _, s := range c.PumpServers {
		s := s
		ins = append(ins, &PumpInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.Specification,

			usedPorts: []int{
				s.Port,
			},
			usedDirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			statusFn: func(_ ...string) string {
				url := fmt.Sprintf("http://%s:%d/status", s.Host, s.Port)
				return statusByURL(url)
			},
		}})
	}
	return ins
}

// PumpInstance represent the Pump instance.
type PumpInstance struct {
	instance
}

// ScaleConfig deploy temporary config on scaling
func (i *PumpInstance) ScaleConfig(e executor.TiOpsExecutor, b *Specification, cluster, user string, paths DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = b

	return i.InitConfig(e, cluster, user, paths)
}

// InitConfig implements Instance interface.
func (i *PumpInstance) InitConfig(e executor.TiOpsExecutor, cluster, user string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, cluster, user, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(PumpSpec)
	cfg := scripts.NewPumpScript(
		i.GetHost()+":"+strconv.Itoa(i.GetPort()),
		i.GetHost(),
		paths.Deploy,
		paths.Data,
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).AppendEndpoints(i.instance.topo.Endpoints(user)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pump_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_pump.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	return i.mergeServerConfig(e, i.topo.ServerConfigs.Pump, spec.Config, paths)
}
