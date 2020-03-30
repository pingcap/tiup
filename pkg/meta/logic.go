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
	"reflect"

	"github.com/google/uuid"
	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/module"
	"github.com/pingcap-incubator/tiops/pkg/template/config"
	"github.com/pingcap-incubator/tiops/pkg/template/scripts"
	system "github.com/pingcap-incubator/tiops/pkg/template/systemd"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
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

// Component represents a component of the cluster.
type Component interface {
	Name() string
	Instances() []Instance
}

// Instance represents the instance.
type Instance interface {
	InstanceSpec
	ID() string
	Ready(executor.TiOpsExecutor) error
	WaitForDown(executor.TiOpsExecutor) error
	InitConfig(executor.TiOpsExecutor, string, string, string) error
	ScaleConfig(executor.TiOpsExecutor, *Specification, string, string, string) error
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
}

// PortStarted wait until a port is being listened
func PortStarted(e executor.TiOpsExecutor, port int) error {
	c := module.WaitForConfig{
		Port:  port,
		State: "started",
	}
	w := module.NewWaitFor(c)
	return w.Execute(e)
}

func PortStopped(e executor.TiOpsExecutor, port int) error {
	c := module.WaitForConfig{
		Port:  port,
		State: "stopped",
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
	topo *Specification

	usedPorts []int
	usedDirs  []string
	statusFn  func(pdHosts ...string) string
}

// Ready implements Instance interface
func (i *instance) Ready(e executor.TiOpsExecutor) error {
	return PortStarted(e, i.port)
}

// WaitForDown implements Instance interface
func (i *instance) WaitForDown(e executor.TiOpsExecutor) error {
	return PortStopped(e, i.port)
}

func (i *instance) InitConfig(e executor.TiOpsExecutor, user, cacheDir, deployDir string) error {
	comp := i.ComponentName()
	port := i.GetPort()
	sysCfg := filepath.Join(cacheDir, fmt.Sprintf("%s-%d.service", comp, port))

	systemCfg := system.NewConfig(comp, user, deployDir)
	// For not auto start if using binlogctl to offline.
	// bad design
	if comp == ComponentPump || comp == ComponentDrainer {
		systemCfg.Restart = "on-failure"
	}

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
func (i *instance) ScaleConfig(e executor.TiOpsExecutor, b *Specification, user, cacheDir, deployDir string) error {
	return i.InitConfig(e, user, cacheDir, deployDir)
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
	return reflect.ValueOf(i.InstanceSpec).FieldByName("DataDir").Interface().(string)
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

// Specification of cluster
type Specification = TopologySpecification

// TiDBComponent represents TiDB component.
type TiDBComponent struct{ *Specification }

// Name implements Component interface.
func (c *TiDBComponent) Name() string {
	return ComponentTiDB
}

// Instances implements Component interface.
func (c *TiDBComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiDBServers))
	for _, s := range c.TiDBServers {
		ins = append(ins, &TiDBInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.Specification,

			usedPorts: []int{
				s.Port,
				s.StatusPort,
			},
			usedDirs: []string{
				s.DeployDir,
			},
			statusFn: s.Status,
		}})
	}
	return ins
}

// TiDBInstance represent the TiDB instance
type TiDBInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *TiDBInstance) InitConfig(e executor.TiOpsExecutor, user, cacheDir, deployDir string) error {
	if err := i.instance.InitConfig(e, user, cacheDir, deployDir); err != nil {
		return err
	}
	ends := []*scripts.PDScript{}
	for _, spec := range i.instance.topo.PDServers {
		ends = append(ends, scripts.NewPDScript(spec.Name, spec.Host, spec.DeployDir, spec.DataDir))
	}
	cfg := scripts.NewTiDBScript(i.GetHost(), deployDir).AppendEndpoints(ends...)
	fp := filepath.Join(cacheDir, fmt.Sprintf("run_tidb_%s.sh", i.GetHost()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(deployDir, "scripts", "run_tidb.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *TiDBInstance) ScaleConfig(e executor.TiOpsExecutor, b *Specification, user, cacheDir, deployDir string) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = b
	return i.InitConfig(e, user, cacheDir, deployDir)
}

// TiKVComponent represents TiKV component.
type TiKVComponent struct {
	*Specification
}

// Name implements Component interface.
func (c *TiKVComponent) Name() string {
	return ComponentTiKV
}

// Instances implements Component interface.
func (c *TiKVComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiKVServers))
	for _, s := range c.TiKVServers {
		ins = append(ins, &TiKVInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.Specification,

			usedPorts: []int{
				s.Port,
				s.StatusPort,
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

// TiKVInstance represent the TiDB instance
type TiKVInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *TiKVInstance) InitConfig(e executor.TiOpsExecutor, user, cacheDir, deployDir string) error {
	if err := i.instance.InitConfig(e, user, cacheDir, deployDir); err != nil {
		return err
	}

	// transfer run script
	ends := []*scripts.PDScript{}
	for _, spec := range i.instance.topo.PDServers {
		ends = append(ends, scripts.NewPDScript(spec.Name, spec.Host, spec.DeployDir, spec.DataDir))
	}
	cfg := scripts.NewTiKVScript(i.GetHost(), deployDir, i.instance.DataDir()).AppendEndpoints(ends...)
	fp := filepath.Join(cacheDir, fmt.Sprintf("run_tikv_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(deployDir, "scripts", "run_tikv.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(cacheDir, fmt.Sprintf("tikv_%s.toml", i.GetHost()))
	if err := config.NewTiKVConfig().ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(deployDir, "conf", "tikv.toml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *TiKVInstance) ScaleConfig(e executor.TiOpsExecutor, b *Specification, user, cacheDir, deployDir string) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = b
	return i.InitConfig(e, user, cacheDir, deployDir)
}

// PDComponent represents PD component.
type PDComponent struct{ *Specification }

// Name implements Component interface.
func (c *PDComponent) Name() string {
	return ComponentPD
}

// Instances implements Component interface.
func (c *PDComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.PDServers))
	for _, s := range c.PDServers {
		ins = append(ins, &PDInstance{
			Name: s.Name,
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.ClientPort,
				sshp:         s.SSHPort,
				topo:         c.Specification,

				usedPorts: []int{
					s.ClientPort,
					s.PeerPort,
				},
				usedDirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				statusFn: s.Status,
			},
		})
	}
	return ins
}

// PDInstance represent the PD instance
type PDInstance struct {
	Name string
	instance
}

// InitConfig implement Instance interface
func (i *PDInstance) InitConfig(e executor.TiOpsExecutor, user, cacheDir, deployDir string) error {
	if err := i.instance.InitConfig(e, user, cacheDir, deployDir); err != nil {
		return err
	}

	ends := []*scripts.PDScript{}
	name := ""
	for _, spec := range i.instance.topo.PDServers {
		if spec.Host == i.GetHost() && spec.ClientPort == i.GetPort() {
			name = spec.Name
		}
		ends = append(ends, scripts.NewPDScript(spec.Name, spec.Host, spec.DeployDir, spec.DataDir))
	}
	cfg := scripts.NewPDScript(name, i.GetHost(), deployDir, i.instance.DataDir()).AppendEndpoints(ends...)
	fp := filepath.Join(cacheDir, fmt.Sprintf("run_pd_%s.sh", i.GetHost()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(deployDir, "scripts", "run_pd.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}
	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *PDInstance) ScaleConfig(e executor.TiOpsExecutor, b *Specification, user, cacheDir, deployDir string) error {
	if err := i.instance.ScaleConfig(e, b, user, cacheDir, deployDir); err != nil {
		return err
	}
	ends := []*scripts.PDScript{}
	name := i.Name
	for _, spec := range b.PDServers {
		if spec.Host == i.GetHost() {
			name = spec.Name
		}
		ends = append(ends, scripts.NewPDScript(spec.Name, spec.Host, spec.DeployDir, spec.DataDir))
	}

	cfg := scripts.NewPDScaleScript(name, i.GetHost(), deployDir, i.instance.DataDir()).AppendEndpoints(ends...)
	fp := filepath.Join(cacheDir, fmt.Sprintf("run_pd_%s_%d.sh", i.GetHost(), i.GetPort()))
	log.Infof("script path: %s", fp)
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(deployDir, "scripts", "run_pd.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}
	return nil
}

// MonitorComponent represents Monitor component.
type MonitorComponent struct{ *Specification }

// Name implements Component interface.
func (c *MonitorComponent) Name() string {
	return ComponentPrometheus
}

// Instances implements Component interface.
func (c *MonitorComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Monitors))
	for _, s := range c.Monitors {
		ins = append(ins, &MonitorInstance{c.Specification, instance{
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
				return "-"
			},
		}})
	}
	return ins
}

// MonitorInstance represent the monitor instance
type MonitorInstance struct {
	topo *Specification
	instance
}

// InitConfig implement Instance interface
func (i *MonitorInstance) InitConfig(e executor.TiOpsExecutor, user, cacheDir, deployDir string) error {
	if err := i.instance.InitConfig(e, user, cacheDir, deployDir); err != nil {
		return err
	}

	// transfer run script
	cfg := scripts.NewPrometheusScript(i.GetHost(), deployDir, filepath.Join(deployDir, "data")).WithPort(uint64(i.GetPort()))
	fp := filepath.Join(cacheDir, fmt.Sprintf("run_prometheus_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(deployDir, "scripts", "run_prometheus.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(cacheDir, fmt.Sprintf("tikv_%s.yml", i.GetHost()))
	// TODO: use real cluster name
	cfig := config.NewPrometheusConfig("test-cluster")
	uniqueHosts := set.NewStringSet()
	for _, pd := range i.topo.PDServers {
		uniqueHosts.Insert(pd.Host)
		cfig.AddPD(pd.Host, uint64(pd.ClientPort))
	}
	for _, kv := range i.topo.TiKVServers {
		uniqueHosts.Insert(kv.Host)
		cfig.AddTiKV(kv.Host, uint64(kv.StatusPort))
	}
	for _, db := range i.topo.TiDBServers {
		uniqueHosts.Insert(db.Host)
		cfig.AddTiDB(db.Host, uint64(db.StatusPort))
	}
	for _, pump := range i.topo.PumpServers {
		uniqueHosts.Insert(pump.Host)
		cfig.AddPump(pump.Host, uint64(pump.Port))
	}
	for _, drainer := range i.topo.Drainers {
		uniqueHosts.Insert(drainer.Host)
		cfig.AddDrainer(drainer.Host, uint64(drainer.Port))
	}
	for _, grafana := range i.topo.Grafana {
		uniqueHosts.Insert(grafana.Host)
		cfig.AddGrafana(grafana.Host, uint64(grafana.Port))
	}
	for host := range uniqueHosts {
		cfig.AddNodeExpoertor(host, uint64(i.topo.MonitoredOptions.NodeExporterPort))
		cfig.AddBlackboxExporter(host, uint64(i.topo.MonitoredOptions.BlackboxExporterPort))
		cfig.AddMonitoredServer(host)
	}

	if err := cfig.ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(deployDir, "conf", "prometheus.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}

// GrafanaComponent represents Grafana component.
type GrafanaComponent struct{ *Specification }

// Name implements Component interface.
func (c *GrafanaComponent) Name() string {
	return ComponentGrafana
}

// Instances implements Component interface.
func (c *GrafanaComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Grafana))
	for _, s := range c.Grafana {
		ins = append(ins, &GrafanaInstance{
			topo: c.Specification,
			instance: instance{
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
				},
				statusFn: func(_ ...string) string {
					return "-"
				},
			},
		})
	}
	return ins
}

// GrafanaInstance represent the grafana instance
type GrafanaInstance struct {
	topo *Specification
	instance
}

// InitConfig implement Instance interface
func (i *GrafanaInstance) InitConfig(e executor.TiOpsExecutor, user, cacheDir, deployDir string) error {
	if err := i.instance.InitConfig(e, user, cacheDir, deployDir); err != nil {
		return err
	}

	// transfer run script
	cfg := scripts.NewGrafanaScript(deployDir)
	fp := filepath.Join(cacheDir, fmt.Sprintf("run_grafana_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(deployDir, "scripts", "run_grafana.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(cacheDir, fmt.Sprintf("grafana_%s.ini", i.GetHost()))
	if err := config.NewGrafanaConfig(i.GetHost(), deployDir).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(deployDir, "conf", "grafana.ini")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer dashboard.yml
	fp = filepath.Join(cacheDir, fmt.Sprintf("dashboard_%s.yml", i.GetHost()))
	if err := config.NewDashboardConfig("test-cluster", deployDir).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(deployDir, "conf", "dashboard.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer datasource.yml
	fp = filepath.Join(cacheDir, fmt.Sprintf("datasource_%s.yml", i.GetHost()))
	if err := config.NewDatasourceConfig("test-cluster", i.GetHost()).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(deployDir, "conf", "datasource.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}

// AlertmanagerComponent represents Alertmanager component.
type AlertmanagerComponent struct{ *Specification }

// Name implements Component interface.
func (c *AlertmanagerComponent) Name() string {
	return ComponentAlertManager
}

// Instances implements Component interface.
func (c *AlertmanagerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Alertmanager))
	for _, s := range c.Alertmanager {
		ins = append(ins, &instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.WebPort,
			sshp:         s.SSHPort,
			topo:         c.Specification,

			usedPorts: []int{
				s.WebPort,
				s.ClusterPort,
			},
			usedDirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			statusFn: func(_ ...string) string {
				return "-"
			},
		})
	}
	return ins
}

// ComponentsByStopOrder return component in the order need to stop.
func (topo *Specification) ComponentsByStopOrder() (comps []Component) {
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
func (topo *Specification) ComponentsByStartOrder() (comps []Component) {
	// "pd", "tikv", "pump", "tidb", "drainer", "prometheus", "grafana", "alertmanager"
	comps = append(comps, &PDComponent{topo})
	comps = append(comps, &TiKVComponent{topo})
	comps = append(comps, &PumpComponent{topo})
	comps = append(comps, &TiDBComponent{topo})
	comps = append(comps, &DrainerComponent{topo})
	comps = append(comps, &MonitorComponent{topo})
	comps = append(comps, &GrafanaComponent{topo})
	comps = append(comps, &AlertmanagerComponent{topo})
	return
}

// IterComponent iterates all components in component starting order
func (topo *Specification) IterComponent(fn func(comp Component)) {
	for _, comp := range topo.ComponentsByStartOrder() {
		fn(comp)
	}
}

// IterInstance iterates all instances in component starting order
func (topo *Specification) IterInstance(fn func(instance Instance)) {
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			fn(inst)
		}
	}
}
