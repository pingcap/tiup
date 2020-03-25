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
	"reflect"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/module"
)

// Components names supported by TiOps
const (
	ComponentTiDB             = "tidb"
	ComponentTiKV             = "tikv"
	ComponentPD               = "pd"
	ComponentGrafana          = "grafana"
	ComponentDrainer          = "drainer"
	ComponentPump             = "pump"
	ComponentAlertManager     = "alertmanager"
	ComponentPrometheus       = "prometheus"
	ComponentPushwaygate      = "pushgateway"
	ComponentBlackboxExporter = "blackbox_exporter"
	ComponentNodeExporter     = "node_exporter"
)

func portStarted(e executor.TiOpsExecutor, port int) error {
	c := module.WaitForConfig{
		Port:  port,
		State: "started",
	}
	w := module.NewWaitFor(c)
	return w.Execute(e)
}

func portStopped(e executor.TiOpsExecutor, port int) error {
	c := module.WaitForConfig{
		Port:  port,
		State: "stopped",
	}
	w := module.NewWaitFor(c)
	return w.Execute(e)
}

type instanceBase struct {
	name string
	host string
	port int
	sshp int
	spec interface{}
}

// Ready implements Instance interface
func (i *instanceBase) Ready(e executor.TiOpsExecutor) error {
	return portStarted(e, i.port)
}

// WaitForDown implements Instance interface
func (i *instanceBase) WaitForDown(e executor.TiOpsExecutor) error {
	return portStopped(e, i.port)
}

// ComponentName implements Instance interface
func (i *instanceBase) ComponentName() string {
	return i.name
}

// InstanceName implements Instance interface
func (i *instanceBase) InstanceName() string {
	if i.port > 0 {
		return fmt.Sprintf("%s%d", i.name, i.port)
	}
	return i.ComponentName()
}

// ServiceName implements Instance interface
func (i *instanceBase) ServiceName() string {
	if i.port > 0 {
		return fmt.Sprintf("%s-%d.service", i.name, i.port)
	}
	return fmt.Sprintf("%s.service", i.name)
}

// GetHost implements Instance interface
func (i *instanceBase) GetHost() string {
	return i.host
}

// GetSSHPort implements Instance interface
func (i *instanceBase) GetSSHPort() int {
	return i.sshp
}

func (i *instanceBase) DeployDir() string {
	return reflect.ValueOf(i.spec).FieldByName("DeployDir").Interface().(string)
}

func (i *instanceBase) GetPort() int {
	return i.port
}

func (i *instanceBase) UUID() string {
	return reflect.ValueOf(i.spec).FieldByName("UUID").Interface().(string)
}

// Specification of cluster
type Specification = TopologySpecification

// TiDBComponent represents TiDB component.
type TiDBComponent []TiDBSpec

// Name implements Component interface.
func (c TiDBComponent) Name() string {
	return ComponentTiDB
}

// Instances implements Component interface.
func (c TiDBComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			port: s.Port,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// TiKVComponent represents TiKV component.
type TiKVComponent []TiKVSpec

// Name implements Component interface.
func (c TiKVComponent) Name() string {
	return ComponentTiKV
}

// Instances implements Component interface.
func (c TiKVComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			port: s.Port,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// PDComponent represents PD component.
type PDComponent []PDSpec

// Name implements Component interface.
func (c PDComponent) Name() string {
	return ComponentPD
}

// Instances implements Component interface.
func (c PDComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			port: s.ClientPort,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// PumpComponent represents Pump component.
type PumpComponent []PumpSpec

// Name implements Component interface.
func (c PumpComponent) Name() string {
	return ComponentPump
}

// Instances implements Component interface.
func (c PumpComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			port: s.Port,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// DrainerComponent represents Drainer component.
type DrainerComponent []DrainerSpec

// Name implements Component interface.
func (c DrainerComponent) Name() string {
	return ComponentDrainer
}

// Instances implements Component interface.
func (c DrainerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			port: s.Port,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// MonitorComponent represents Monitor component.
type MonitorComponent []PrometheusSpec

// Name implements Component interface.
func (c MonitorComponent) Name() string {
	return ComponentPrometheus
}

// Instances implements Component interface.
func (c MonitorComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			port: s.Port,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// GrafanaComponent represents Grafana component.
type GrafanaComponent []GrafanaSpec

// Name implements Component interface.
func (c GrafanaComponent) Name() string {
	return ComponentGrafana
}

// Instances implements Component interface.
func (c GrafanaComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			port: s.Port,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// AlertmanagerComponent represents Alertmanager component.
type AlertmanagerComponent []AlertManagerSpec

// Name implements Component interface.
func (c AlertmanagerComponent) Name() string {
	return ComponentAlertManager
}

// Instances implements Component interface.
func (c AlertmanagerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c))
	for _, s := range c {
		ins = append(ins, &instanceBase{
			name: c.Name(),
			host: s.Host,
			sshp: s.SSHPort,
			spec: s,
		})
	}
	return ins
}

// ComponentsByStartOrder return component in the order need to start.
func (s *Specification) ComponentsByStartOrder() (comps []Component) {
	// "pd", "tikv", "pump", "tidb", "drainer", "prometheus", "grafana", "alertmanager"

	comps = append(comps, PDComponent(s.PDServers))
	comps = append(comps, TiKVComponent(s.TiKVServers))
	comps = append(comps, PumpComponent(s.PumpServers))
	comps = append(comps, TiDBComponent(s.TiDBServers))
	comps = append(comps, DrainerComponent(s.Drainers))
	comps = append(comps, MonitorComponent(s.MonitorSpec))
	comps = append(comps, GrafanaComponent(s.Grafana))
	comps = append(comps, AlertmanagerComponent(s.Alertmanager))

	return
}

// Component represents a component of the cluster.
type Component interface {
	Name() string
	Instances() []Instance
}

// Instance represents the instance.
type Instance interface {
	Ready(executor.TiOpsExecutor) error
	WaitForDown(executor.TiOpsExecutor) error
	ComponentName() string
	InstanceName() string
	ServiceName() string
	GetHost() string
	GetPort() int
	GetSSHPort() int
	DeployDir() string
	UUID() string
}
