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

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/module"
	system "github.com/pingcap/tiup/pkg/cluster/template/systemd"
	"github.com/pingcap/tiup/pkg/meta"
)

// Components names supported by TiOps
const (
	ComponentTiDB             = "tidb"
	ComponentTiKV             = "tikv"
	ComponentPD               = "pd"
	ComponentTiFlash          = "tiflash"
	ComponentGrafana          = "grafana"
	ComponentDrainer          = "drainer"
	ComponentPump             = "pump"
	ComponentCDC              = "cdc"
	ComponentTiSpark          = "tispark"
	ComponentSpark            = "spark"
	ComponentAlertManager     = "alertmanager"
	ComponentPrometheus       = "prometheus"
	ComponentPushwaygate      = "pushgateway"
	ComponentBlackboxExporter = "blackbox_exporter"
	ComponentNodeExporter     = "node_exporter"
	ComponentCheckCollector   = "insight"
)

// Component represents a component of the cluster.
type Component interface {
	Name() string
	Instances() []Instance
}

// RollingUpdateInstance represent a instance need to transfer state when restart.
// e.g transfer leader.
type RollingUpdateInstance interface {
	PreRestart(topo Topology, apiTimeoutSeconds int) error
	PostRestart(topo Topology) error
}

// Instance represents the instance.
type Instance interface {
	InstanceSpec
	ID() string
	Ready(executor.Executor, int64) error
	WaitForDown(executor.Executor, int64) error
	InitConfig(e executor.Executor, clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error
	ScaleConfig(e executor.Executor, topo Topology, clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error
	PrepareStart() error
	ComponentName() string
	InstanceName() string
	ServiceName() string
	GetHost() string
	GetPort() int
	GetSSHPort() int
	DeployDir() string
	UsedPorts() []int
	UsedDirs() []string
	Status(pdList ...string) string
	DataDir() string
	LogDir() string
	OS() string // only linux supported now
	Arch() string
}

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

	name       string
	host       string
	listenHost string
	port       int
	sshp       int
	topo       *Specification

	usedPorts []int
	usedDirs  []string
	statusFn  func(pdHosts ...string) string
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

	resource := MergeResourceControl(i.topo.GlobalOptions.ResourceControl, i.resourceControl())
	systemCfg := system.NewConfig(comp, user, paths.Deploy).
		WithMemoryLimit(resource.MemoryLimit).
		WithCPUQuota(resource.CPUQuota).
		WithIOReadBandwidthMax(resource.IOReadBandwidthMax).
		WithIOWriteBandwidthMax(resource.IOWriteBandwidthMax)

	// For not auto start if using binlogctl to offline.
	// bad design
	if comp == ComponentPump || comp == ComponentDrainer {
		systemCfg.Restart = "on-failure"
	}

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return errors.Trace(err)
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := e.Transfer(sysCfg, tgt, false); err != nil {
		return errors.Annotatef(err, "transfer from %s to %s failed", sysCfg, tgt)
	}
	cmd := fmt.Sprintf("mv %s /etc/systemd/system/%s-%d.service", tgt, comp, port)
	if _, _, err := e.Execute(cmd, true); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	return nil
}

// mergeServerConfig merges the server configuration and overwrite the global configuration
func (i *instance) mergeServerConfig(e executor.Executor, globalConf, instanceConf map[string]interface{}, paths meta.DirPaths) error {
	fp := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.toml", i.ComponentName(), i.GetHost(), i.GetPort()))
	conf, err := merge2Toml(i.ComponentName(), globalConf, instanceConf)
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

// mergeTiFlashLearnerServerConfig merges the server configuration and overwrite the global configuration
func (i *instance) mergeTiFlashLearnerServerConfig(e executor.Executor, globalConf, instanceConf map[string]interface{}, paths meta.DirPaths) error {
	fp := filepath.Join(paths.Cache, fmt.Sprintf("%s-learner-%s-%d.toml", i.ComponentName(), i.GetHost(), i.GetPort()))
	conf, err := merge2Toml(i.ComponentName()+"-learner", globalConf, instanceConf)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fp, conf, os.ModePerm)
	if err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", fmt.Sprintf("%s-learner.toml", i.ComponentName()))
	// transfer config
	return e.Transfer(fp, dst, false)
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
	switch i.ComponentName() {
	case ComponentSpark, ComponentTiSpark:
		if i.port > 0 {
			return fmt.Sprintf("%s-%d.service", i.Role(), i.port)
		}
		return fmt.Sprintf("%s.service", i.Role())
	}
	if i.port > 0 {
		return fmt.Sprintf("%s-%d.service", i.name, i.port)
	}
	return fmt.Sprintf("%s.service", i.name)
}

// GetHost implements Instance interface
func (i *instance) GetHost() string {
	return i.host
}

// GetListenHost implements Instance interface
func (i *instance) GetListenHost() string {
	if i.listenHost == "" {
		return "0.0.0.0"
	}
	return i.listenHost
}

// GetSSHPort implements Instance interface
func (i *instance) GetSSHPort() int {
	return i.sshp
}

func (i *instance) DeployDir() string {
	return reflect.ValueOf(i.InstanceSpec).FieldByName("DeployDir").String()
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

func (i *instance) OS() string {
	v := reflect.ValueOf(i.InstanceSpec).FieldByName("OS")
	if !v.IsValid() {
		return ""
	}
	return v.Interface().(string)
}

func (i *instance) Arch() string {
	v := reflect.ValueOf(i.InstanceSpec).FieldByName("Arch")
	if !v.IsValid() {
		return ""
	}
	return v.Interface().(string)
}

// PrepareStart checks instance requirements before starting
func (i *instance) PrepareStart() error {
	return nil
}

// MergeResourceControl merge the rhs into lhs and overwrite rhs if lhs has value for same field
func MergeResourceControl(lhs, rhs meta.ResourceControl) meta.ResourceControl {
	if rhs.MemoryLimit != "" {
		lhs.MemoryLimit = rhs.MemoryLimit
	}
	if rhs.CPUQuota != "" {
		lhs.CPUQuota = rhs.CPUQuota
	}
	if rhs.IOReadBandwidthMax != "" {
		lhs.IOReadBandwidthMax = rhs.IOReadBandwidthMax
	}
	if rhs.IOWriteBandwidthMax != "" {
		lhs.IOWriteBandwidthMax = rhs.IOWriteBandwidthMax
	}
	return lhs
}

func (i *instance) resourceControl() meta.ResourceControl {
	if v := reflect.ValueOf(i.InstanceSpec).FieldByName("ResourceControl"); v.IsValid() {
		return v.Interface().(meta.ResourceControl)
	}
	return meta.ResourceControl{}
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

func (i *instance) Status(pdList ...string) string {
	return i.statusFn(pdList...)
}
