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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	system "github.com/pingcap/tiup/pkg/cluster/template/systemd"
)

// Components names supported by TiOps
const (
	ComponentDMMaster     = "dm-master"
	ComponentDMWorker     = "dm-worker"
	ComponentDumpling     = "dumpling"
	ComponentLightning    = "lightning"
	ComponentImporter     = "importer"
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

// PortStarted wait until a port is being listened
func PortStarted(e executor.Executor, port int, timeout int64) error {
	c := module.WaitForConfig{
		Port:    port,
		State:   "started",
		Timeout: time.Second * time.Duration(timeout),
	}
	w := module.NewWaitFor(c)
	return w.Execute(e)
}

// PortStopped wait until a port is being released
func PortStopped(e executor.Executor, port int, timeout int64) error {
	c := module.WaitForConfig{
		Port:    port,
		State:   "stopped",
		Timeout: time.Second * time.Duration(timeout),
	}
	w := module.NewWaitFor(c)
	return w.Execute(e)
}

type instance struct {
	InstanceSpec

	name string
	host string
	port int
	sshp int
	topo *Topology

	usedPorts []int
	usedDirs  []string
	statusFn  func(masterHosts ...string) string
}

// Ready implements Instance interface
func (i *instance) Ready(e executor.Executor, timeout int64) error {
	return PortStarted(e, i.port, timeout)
}

// WaitForDown implements Instance interface
func (i *instance) WaitForDown(e executor.Executor, timeout int64) error {
	return PortStopped(e, i.port, timeout)
}

func (i *instance) InitConfig(e executor.Executor, _, _, user string, paths meta.DirPaths) error {
	comp := i.ComponentName()
	host := i.GetHost()
	port := i.GetPort()
	sysCfg := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, host, port))

	resource := spec.MergeResourceControl(i.topo.GlobalOptions.ResourceControl, i.resourceControl())
	systemCfg := system.NewConfig(comp, user, paths.Deploy).
		WithMemoryLimit(resource.MemoryLimit).
		WithCPUQuota(resource.CPUQuota).
		WithIOReadBandwidthMax(resource.IOReadBandwidthMax).
		WithIOWriteBandwidthMax(resource.IOWriteBandwidthMax)

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return err
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := e.Transfer(sysCfg, tgt, false); err != nil {
		return err
	}
	cmd := fmt.Sprintf("mv %s /etc/systemd/system/%s-%d.service", tgt, comp, port)
	if _, _, err := e.Execute(cmd, true); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *instance) ScaleConfig(e executor.Executor, _ *Topology, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// ID returns the identifier of this instance, the ID is constructed by host:port
func (i *instance) ID() string {
	return fmt.Sprintf("%s:%d", i.host, i.port)
}

// ComponentName implements Instance interface
func (i *instance) ComponentName() string {
	return i.name
}

// InstanceName implements Instance interface
func (i *instance) InstanceName() string {
	if i.port > 0 {
		return fmt.Sprintf("%s%d", i.name, i.port)
	}
	return i.ComponentName()
}

// ServiceName implements Instance interface
func (i *instance) ServiceName() string {
	if i.port > 0 {
		return fmt.Sprintf("%s-%d.service", i.name, i.port)
	}
	return fmt.Sprintf("%s.service", i.name)
}

// GetHost implements Instance interface
func (i *instance) GetHost() string {
	return i.host
}

// GetSSHPort implements Instance interface
func (i *instance) GetSSHPort() int {
	return i.sshp
}

func (i *instance) DeployDir() string {
	return reflect.ValueOf(i.InstanceSpec).FieldByName("DeployDir").Interface().(string)
}

func (i *instance) DataDir() string {
	dataDir := reflect.ValueOf(i.InstanceSpec).FieldByName("DataDir")
	if !dataDir.IsValid() {
		return ""
	}

	// the default data_dir is relative to deploy_dir
	if dataDir.String() != "" && !strings.HasPrefix(dataDir.String(), "/") {
		return filepath.Join(i.DeployDir(), dataDir.String())
	}

	return dataDir.String()
}

func (i *instance) resourceControl() meta.ResourceControl {
	return reflect.ValueOf(i.InstanceSpec).
		FieldByName("ResourceControl").
		Interface().(meta.ResourceControl)
}

func (i *instance) OS() string {
	return reflect.ValueOf(i.InstanceSpec).FieldByName("OS").Interface().(string)
}

func (i *instance) Arch() string {
	return reflect.ValueOf(i.InstanceSpec).FieldByName("Arch").Interface().(string)
}

func (i *instance) PrepareStart() error {
	return nil
}

func (i *instance) LogDir() string {
	logDir := ""

	field := reflect.ValueOf(i.InstanceSpec).FieldByName("LogDir")
	if field.IsValid() {
		logDir = field.Interface().(string)
	}

	if logDir == "" {
		logDir = "log"
	}
	if !strings.HasPrefix(logDir, "/") {
		logDir = filepath.Join(i.DeployDir(), logDir)
	}
	return logDir
}

func (i *instance) GetPort() int {
	return i.port
}

func (i *instance) UsedPorts() []int {
	return i.usedPorts
}

func (i *instance) UsedDirs() []string {
	return i.usedDirs
}

func (i *instance) Status(masterList ...string) string {
	return i.statusFn(masterList...)
}

// DMMasterComponent represents TiDB component.
type DMMasterComponent struct{ *Topology }

// Name implements Component interface.
func (c *DMMasterComponent) Name() string {
	return ComponentDMMaster
}

// Instances implements Component interface.
func (c *DMMasterComponent) Instances() []Instance {
	ins := make([]Instance, 0)
	for _, s := range c.Masters {
		s := s
		ins = append(ins, &DMMasterInstance{
			Name: s.Name,
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.Port,
				sshp:         s.SSHPort,
				topo:         c.Topology,

				usedPorts: []int{
					s.Port,
					s.PeerPort,
				},
				usedDirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				statusFn: s.Status,
			}})
	}
	return ins
}

// DMMasterInstance represent the TiDB instance
type DMMasterInstance struct {
	Name string
	instance
}

// InitConfig implement Instance interface
func (i *DMMasterInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(MasterSpec)
	cfg := scripts.NewDMMasterScript(
		spec.Name,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).WithPeerPort(spec.PeerPort).AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)

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
	return i.mergeServerConfig(e, i.topo.ServerConfigs.Master, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *DMMasterInstance) ScaleConfig(e executor.Executor, topo spec.Topology, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
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

// Instances implements Component interface.
func (c *DMWorkerComponent) Instances() []Instance {
	ins := make([]Instance, 0)
	for _, s := range c.Workers {
		s := s
		ins = append(ins, &DMWorkerInstance{
			Name: s.Name,
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.Port,
				sshp:         s.SSHPort,
				topo:         c.Topology,

				usedPorts: []int{
					s.Port,
				},
				usedDirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				statusFn: s.Status,
			}})
	}

	return ins
}

// DMWorkerInstance represent the DM worker instance
type DMWorkerInstance struct {
	Name string
	instance
}

// InitConfig implement Instance interface
func (i *DMWorkerInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(WorkerSpec)
	cfg := scripts.NewDMWorkerScript(
		i.Name,
		i.GetHost(),
		paths.Deploy,
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)
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
	return i.mergeServerConfig(e, i.topo.ServerConfigs.Worker, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *DMWorkerInstance) ScaleConfig(e executor.Executor, topo spec.Topology, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = topo.(*Topology)
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
	// "dm-master", "dm-worker", "dm-portal"
	comps = append(comps, &DMMasterComponent{topo})
	comps = append(comps, &DMWorkerComponent{topo})
	return
}

// ComponentsByUpdateOrder return component in the order need to be updated.
func (topo *Topology) ComponentsByUpdateOrder() (comps []Component) {
	// "dm-master", "dm-worker", "dm-portal"
	comps = append(comps, &DMMasterComponent{topo})
	comps = append(comps, &DMWorkerComponent{topo})
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
	for _, spec := range topo.Masters {
		deployDir := clusterutil.Abs(user, spec.DeployDir)
		// data dir would be empty for components which don't need it
		dataDir := spec.DataDir
		// the default data_dir is relative to deploy_dir
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join(deployDir, dataDir)
		}
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(user, spec.LogDir)

		script := scripts.NewDMMasterScript(
			spec.Name,
			spec.Host,
			deployDir,
			dataDir,
			logDir).
			WithPort(spec.Port).
			WithPeerPort(spec.PeerPort)
		ends = append(ends, script)
	}
	return ends
}

// mergeServerConfig merges the server configuration and overwrite the global configuration
func (i *instance) mergeServerConfig(e executor.Executor, globalConf, instanceConf map[string]interface{}, paths meta.DirPaths) error {
	fp := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.toml", i.ComponentName(), i.GetHost(), i.GetPort()))
	conf, err := spec.Merge2Toml(i.ComponentName(), globalConf, instanceConf)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fp, conf, os.ModePerm)
	if err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", fmt.Sprintf("%s.toml", i.ComponentName()))
	// transfer config
	return e.Transfer(fp, dst, false)
}
