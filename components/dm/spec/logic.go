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

package spec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"

	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
)

// Components names supported by TiOps
const (
	ComponentDMMaster     = "dm-master"
	ComponentDMWorker     = "dm-worker"
	ComponentPrometheus   = spec.ComponentPrometheus
	ComponentGrafana      = spec.ComponentGrafana
	ComponentAlertManager = spec.ComponentAlertManager
)

type (
	// InstanceSpec represent a instance specification
	InstanceSpec interface {
		Role() string
		SSH() (string, int)
		GetMainPort() int
		IsImported() bool
	}
)

// Component represents a component of the cluster.
type Component = spec.Component

// Instance represents an instance
type Instance = spec.Instance

// DMMasterComponent represents TiDB component.
type DMMasterComponent struct{ *Topology }

// Name implements Component interface.
func (c *DMMasterComponent) Name() string {
	return ComponentDMMaster
}

// Role implements Component interface.
func (c *DMMasterComponent) Role() string {
	return ComponentDMMaster
}

// Instances implements Component interface.
func (c *DMMasterComponent) Instances() []Instance {
	ins := make([]Instance, 0)
	for _, s := range c.Masters {
		s := s
		ins = append(ins, &MasterInstance{
			Name: s.Name,
			BaseInstance: spec.BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				Port:         s.Port,
				SSHP:         s.SSHPort,

				Ports: []int{
					s.Port,
					s.PeerPort,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: s.Status,
			},
			topo: c.Topology,
		})
	}
	return ins
}

// MasterInstance represent the TiDB instance
type MasterInstance struct {
	Name string
	spec.BaseInstance
	topo *Topology
}

// InitConfig implement Instance interface
func (i *MasterInstance) InitConfig(
	e executor.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(MasterSpec)
	cfg := scripts.NewDMMasterScript(
		spec.Name,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).WithPeerPort(spec.PeerPort).AppendEndpoints(i.topo.Endpoints(deployUser)...).WithV1SourcePath(spec.V1SourcePath)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_dm-master_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_dm-master.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	specConfig := spec.Config
	return i.MergeServerConfig(e, i.topo.ServerConfigs.Master, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *MasterInstance) ScaleConfig(
	e executor.Executor,
	topo spec.Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	if err := i.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	c := topo.(*Topology)
	spec := i.InstanceSpec.(MasterSpec)
	cfg := scripts.NewDMMasterScaleScript(
		spec.Name,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).WithPeerPort(spec.PeerPort).AppendEndpoints(c.Endpoints(deployUser)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_dm-master_%s_%d.sh", i.GetHost(), i.GetPort()))
	log.Infof("script path: %s", fp)
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_dm-master.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	return nil
}

// DMWorkerComponent represents DM worker component.
type DMWorkerComponent struct {
	*Topology
}

// Name implements Component interface.
func (c *DMWorkerComponent) Name() string {
	return ComponentDMWorker
}

// Role implements Component interface.
func (c *DMWorkerComponent) Role() string {
	return ComponentDMWorker
}

// Instances implements Component interface.
func (c *DMWorkerComponent) Instances() []Instance {
	ins := make([]Instance, 0)
	for _, s := range c.Workers {
		s := s
		ins = append(ins, &WorkerInstance{
			Name: s.Name,
			BaseInstance: spec.BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				Port:         s.Port,
				SSHP:         s.SSHPort,

				Ports: []int{
					s.Port,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: s.Status,
			},
			topo: c.Topology,
		})
	}

	return ins
}

// WorkerInstance represent the DM worker instance
type WorkerInstance struct {
	Name string
	spec.BaseInstance
	topo *Topology
}

// InitConfig implement Instance interface
func (i *WorkerInstance) InitConfig(
	e executor.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(WorkerSpec)
	cfg := scripts.NewDMWorkerScript(
		i.Name,
		i.GetHost(),
		paths.Deploy,
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).AppendEndpoints(i.topo.Endpoints(deployUser)...)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_dm-worker_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_dm-worker.sh")

	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	specConfig := spec.Config
	return i.MergeServerConfig(e, i.topo.ServerConfigs.Worker, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *WorkerInstance) ScaleConfig(
	e executor.Executor,
	topo spec.Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() {
		i.topo = s
	}()
	i.topo = topo.(*Topology)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// GetGlobalOptions returns cluster topology
func (topo *Topology) GetGlobalOptions() spec.GlobalOptions {
	return topo.GlobalOptions
}

// GetMonitoredOptions returns MonitoredOptions
func (topo *Topology) GetMonitoredOptions() *spec.MonitoredOptions {
	return nil
}

// ComponentsByStopOrder return component in the order need to stop.
func (topo *Topology) ComponentsByStopOrder() (comps []Component) {
	comps = topo.ComponentsByStartOrder()
	// revert order
	i := 0
	j := len(comps) - 1
	for i < j {
		comps[i], comps[j] = comps[j], comps[i]
		i++
		j--
	}
	return
}

// ComponentsByStartOrder return component in the order need to start.
func (topo *Topology) ComponentsByStartOrder() (comps []Component) {
	// "dm-master", "dm-worker"
	comps = append(comps, &DMMasterComponent{topo})
	comps = append(comps, &DMWorkerComponent{topo})
	comps = append(comps, &MonitorComponent{topo})
	comps = append(comps, &GrafanaComponent{topo})
	comps = append(comps, &AlertManagerComponent{topo})
	return
}

// ComponentsByUpdateOrder return component in the order need to be updated.
func (topo *Topology) ComponentsByUpdateOrder() (comps []Component) {
	// "dm-master", "dm-worker"
	comps = append(comps, &DMMasterComponent{topo})
	comps = append(comps, &DMWorkerComponent{topo})
	comps = append(comps, &MonitorComponent{topo})
	comps = append(comps, &GrafanaComponent{topo})
	comps = append(comps, &AlertManagerComponent{topo})
	return
}

// IterComponent iterates all components in component starting order
func (topo *Topology) IterComponent(fn func(comp Component)) {
	for _, comp := range topo.ComponentsByStartOrder() {
		fn(comp)
	}
}

// IterInstance iterates all instances in component starting order
func (topo *Topology) IterInstance(fn func(instance Instance)) {
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			fn(inst)
		}
	}
}

// IterHost iterates one instance for each host
func (topo *Topology) IterHost(fn func(instance Instance)) {
	hostMap := make(map[string]bool)
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			host := inst.GetHost()
			_, ok := hostMap[host]
			if !ok {
				hostMap[host] = true
				fn(inst)
			}
		}
	}
}

// Endpoints returns the PD endpoints configurations
func (topo *Topology) Endpoints(user string) []*scripts.DMMasterScript {
	var ends []*scripts.DMMasterScript
	for _, s := range topo.Masters {
		deployDir := spec.Abs(user, s.DeployDir)
		// data dir would be empty for components which don't need it
		dataDir := s.DataDir
		// the default data_dir is relative to deploy_dir
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join(deployDir, dataDir)
		}
		// log dir will always be with values, but might not used by the component
		logDir := spec.Abs(user, s.LogDir)

		script := scripts.NewDMMasterScript(
			s.Name,
			s.Host,
			deployDir,
			dataDir,
			logDir).
			WithPort(s.Port).
			WithPeerPort(s.PeerPort)
		ends = append(ends, script)
	}
	return ends
}
