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

	"github.com/pingcap-incubator/tiup/pkg/cluster/executor"
	"github.com/pingcap-incubator/tiup/pkg/cluster/template/scripts"
)

// CDCComponent represents CDC component.
type CDCComponent struct{ *ClusterSpecification }

// Name implements Component interface.
func (c *CDCComponent) Name() string {
	return ComponentCDC
}

// Instances implements Component interface.
func (c *CDCComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.CDCServers))
	for _, s := range c.CDCServers {
		s := s
		ins = append(ins, &CDCInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.ClusterSpecification,

			usedPorts: []int{
				s.Port,
			},
			usedDirs: []string{
				s.DeployDir,
			},
			statusFn: func(_ ...string) string {
				url := fmt.Sprintf("http://%s:%d/status", s.Host, s.Port)
				return statusByURL(url)
			},
		}})
	}
	return ins
}

// CDCInstance represent the CDC instance.
type CDCInstance struct {
	instance
}

// ScaleConfig deploy temporary config on scaling
func (i *CDCInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification, clusterName, clusterVersion, user string, paths DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = b.GetClusterSpecification()

	return i.InitConfig(e, clusterName, clusterVersion, user, paths)
}

// InitConfig implements Instance interface.
func (i *CDCInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(CDCSpec)
	cfg := scripts.NewCDCScript(
		i.GetHost(),
		paths.Deploy,
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_cdc_%s_%d.sh", i.GetHost(), i.GetPort()))

	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_cdc.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	specConfig := spec.Config

	return i.mergeServerConfig(e, i.topo.ServerConfigs.CDC, specConfig, paths)
}
