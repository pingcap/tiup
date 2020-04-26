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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/pingcap-incubator/tiup-cluster/pkg/clusterutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/executor"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/module"
	"github.com/pingcap-incubator/tiup-cluster/pkg/template/config"
	"github.com/pingcap-incubator/tiup-cluster/pkg/template/scripts"
	system "github.com/pingcap-incubator/tiup-cluster/pkg/template/systemd"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap/errors"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"
)

// Components names supported by TiOps
const (
	ComponentDMMaster         = "dm_master"
	ComponentDMWorker         = "dm_worker"
	ComponentTiDB             = "tidb"
	ComponentTiKV             = "tikv"
	ComponentPD               = "pd"
	ComponentTiFlash          = "tiflash"
	ComponentGrafana          = "grafana"
	ComponentDrainer          = "drainer"
	ComponentPump             = "pump"
	ComponentCDC              = "cdc"
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
	InitConfig(e executor.TiOpsExecutor, clusterName string, clusterVersion string, deployUser string, paths DirPaths) error
	ScaleConfig(e executor.TiOpsExecutor, topo Specification, clusterName string, clusterVersion string, deployUser string, paths DirPaths) error
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
}

// Specification represents the topology of cluster/dm
type Specification interface {
	ComponentsByStopOrder() (comps []Component)
	ComponentsByStartOrder() (comps []Component)
	IterComponent(fn func(comp Component))
	IterInstance(fn func(instance Instance))
	IterHost(fn func(instance Instance))
	GetGlobalOptions() GlobalOptions
	GetMonitoredOptions() MonitoredOptions
	GetClusterSpecification() *ClusterSpecification
	GetDMSpecification() *DMSpecification
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

// PortStopped wait until a port is being released
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
	topo *ClusterSpecification

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

func (i *instance) InitConfig(e executor.TiOpsExecutor, _, _, user string, paths DirPaths) error {
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

// mergeServerConfig merges the server configuration and overwrite the global configuration
func (i *instance) mergeServerConfig(e executor.TiOpsExecutor, globalConf, instanceConf map[string]interface{}, paths DirPaths) error {
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
func (i *instance) mergeTiFlashLearnerServerConfig(e executor.TiOpsExecutor, globalConf, instanceConf map[string]interface{}, paths DirPaths) error {
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
	return dataDir.Interface().(string)
}

// MergeResourceControl merge the rhs into lhs and overwrite rhs if lhs has value for same field
func MergeResourceControl(lhs, rhs ResourceControl) ResourceControl {
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

func (i *instance) resourceControl() ResourceControl {
	return reflect.ValueOf(i.InstanceSpec).
		FieldByName("ResourceControl").
		Interface().(ResourceControl)
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

func (i *instance) Status(pdList ...string) string {
	return i.statusFn(pdList...)
}

// ClusterSpecification of cluster
type ClusterSpecification = TopologySpecification

// TiDBComponent represents TiDB component.
type TiDBComponent struct{ *ClusterSpecification }

// Name implements Component interface.
func (c *TiDBComponent) Name() string {
	return ComponentTiDB
}

// Instances implements Component interface.
func (c *TiDBComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiDBServers))
	for _, s := range c.TiDBServers {
		s := s
		ins = append(ins, &TiDBInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.ClusterSpecification,

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
func (i *TiDBInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(TiDBSpec)
	cfg := scripts.NewTiDBScript(
		i.GetHost(),
		paths.Deploy,
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).WithStatusPort(spec.StatusPort).AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tidb_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_tidb.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	specConfig := spec.Config
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			"config",
			fmt.Sprintf(
				"%s-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		mergedConfig, err := mergeImported(importConfig, spec.Config)
		if err != nil {
			return err
		}
		specConfig = mergedConfig
	}

	return i.mergeServerConfig(e, i.instance.topo.ServerConfigs.TiDB, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiDBInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = b.GetClusterSpecification()
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// TiKVComponent represents TiKV component.
type TiKVComponent struct {
	*ClusterSpecification
}

// Name implements Component interface.
func (c *TiKVComponent) Name() string {
	return ComponentTiKV
}

// Instances implements Component interface.
func (c *TiKVComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiKVServers))
	for _, s := range c.TiKVServers {
		s := s
		ins = append(ins, &TiKVInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.ClusterSpecification,

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
func (i *TiKVInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(TiKVSpec)
	cfg := scripts.NewTiKVScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data,
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).WithStatusPort(spec.StatusPort).AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tikv_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tikv.sh")

	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	specConfig := spec.Config
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			"config",
			fmt.Sprintf(
				"%s-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		mergedConfig, err := mergeImported(importConfig, spec.Config)
		if err != nil {
			return err
		}
		specConfig = mergedConfig
	}

	return i.mergeServerConfig(e, i.instance.topo.ServerConfigs.TiKV, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiKVInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = b.GetClusterSpecification()
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// PDComponent represents PD component.
type PDComponent struct{ *ClusterSpecification }

// Name implements Component interface.
func (c *PDComponent) Name() string {
	return ComponentPD
}

// Instances implements Component interface.
func (c *PDComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.PDServers))
	for _, s := range c.PDServers {
		s := s
		ins = append(ins, &PDInstance{
			Name: s.Name,
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.ClientPort,
				sshp:         s.SSHPort,
				topo:         c.ClusterSpecification,

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
func (i *PDInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	var name string
	for _, spec := range i.instance.topo.PDServers {
		if spec.Host == i.GetHost() && spec.ClientPort == i.GetPort() {
			name = spec.Name
		}
	}

	spec := i.InstanceSpec.(PDSpec)
	cfg := scripts.NewPDScript(
		name,
		i.GetHost(),
		paths.Deploy,
		paths.Data,
		paths.Log,
	).WithClientPort(spec.ClientPort).WithPeerPort(spec.PeerPort).AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pd_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_pd.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// Set the PD metrics storage address
	if semver.Compare(clusterVersion, "v3.1.0") >= 0 && len(i.instance.topo.Monitors) > 0 {
		if spec.Config == nil {
			spec.Config = map[string]interface{}{}
		}
		prom := i.instance.topo.Monitors[0]
		spec.Config["pd-server.metric-storage"] = fmt.Sprintf("http://%s:%d", prom.Host, prom.Port)
	}

	specConfig := spec.Config
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			"config",
			fmt.Sprintf(
				"%s-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		mergedConfig, err := mergeImported(importConfig, spec.Config)
		if err != nil {
			return err
		}
		specConfig = mergedConfig
	}

	return i.mergeServerConfig(e, i.instance.topo.ServerConfigs.PD, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *PDInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	c := b.GetClusterSpecification()
	name := i.Name
	for _, spec := range c.PDServers {
		if spec.Host == i.GetHost() {
			name = spec.Name
		}
	}

	spec := i.InstanceSpec.(PDSpec)
	cfg := scripts.NewPDScaleScript(
		name,
		i.GetHost(),
		paths.Deploy,
		paths.Data,
		paths.Log,
	).WithPeerPort(spec.PeerPort).WithNumaNode(spec.NumaNode).WithClientPort(spec.ClientPort).AppendEndpoints(c.Endpoints(deployUser)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pd_%s_%d.sh", i.GetHost(), i.GetPort()))
	log.Infof("script path: %s", fp)
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_pd.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}
	return nil
}

// TiFlashComponent represents TiFlash component.
type TiFlashComponent struct{ *ClusterSpecification }

// Name implements Component interface.
func (c *TiFlashComponent) Name() string {
	return ComponentTiFlash
}

// Instances implements Component interface.
func (c *TiFlashComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiFlashServers))
	for _, s := range c.TiFlashServers {
		ins = append(ins, &TiFlashInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.TCPPort,
			sshp:         s.SSHPort,
			topo:         c.ClusterSpecification,

			usedPorts: []int{
				s.TCPPort,
				s.HTTPPort,
				s.FlashServicePort,
				s.FlashProxyPort,
				s.FlashProxyStatusPort,
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

// TiFlashInstance represent the TiFlash instance
type TiFlashInstance struct {
	instance
}

// InitTiFlashConfig initializes TiFlash config file
func (i *TiFlashInstance) InitTiFlashConfig(cfg *scripts.TiFlashScript, src map[string]interface{}) (map[string]interface{}, error) {
	topo := TopologySpecification{}

	err := yaml.Unmarshal([]byte(fmt.Sprintf(`
server_configs:
  tiflash:
    default_profile: "default"
    display_name: "TiFlash"
    listen_host: "0.0.0.0"
    mark_cache_size: 5368709120
    tmp_path: "%[11]s"
    path: "%[1]s"
    tcp_port: %[3]d
    http_port: %[4]d
    flash.tidb_status_addr: "%[5]s"
    flash.service_addr: "%[6]s:%[7]d"
    flash.flash_cluster.cluster_manager_path: "%[10]s/bin/tiflash/flash_cluster_manager"
    flash.flash_cluster.log: "%[2]s/tiflash_cluster_manager.log"
    flash.flash_cluster.master_ttl: 60
    flash.flash_cluster.refresh_interval: 20
    flash.flash_cluster.update_rule_interval: 5
    flash.proxy.config: "%[10]s/conf/tiflash-learner.toml"
    status.metrics_port: %[8]d
    logger.errorlog: "%[2]s/tiflash_error.log"
    logger.log: "%[2]s/tiflash.log"
    logger.count: 20
    logger.level: "debug"
    logger.size: "1000M"
    application.runAsDaemon: true
    raft.pd_addr: "%[9]s"
    quotas.default.interval.duration: 3600
    quotas.default.interval.errors: 0
    quotas.default.interval.execution_time: 0
    quotas.default.interval.queries: 0
    quotas.default.interval.read_rows: 0
    quotas.default.interval.result_rows: 0
    users.default.password: ""
    users.default.profile: "default"
    users.default.quota: "default"
    users.default.networks.ip: "::/0"
    users.readonly.password: ""
    users.readonly.profile: "readonly"
    users.readonly.quota: "default"
    users.readonly.networks.ip: "::/0"
    profiles.default.load_balancing: "random"
    profiles.default.max_memory_usage: 10000000000
    profiles.default.use_uncompressed_cache: 0
    profiles.readonly.readonly: 1
`, cfg.DataDir, cfg.LogDir, cfg.TCPPort, cfg.HTTPPort, cfg.TiDBStatusAddrs, cfg.IP, cfg.FlashServicePort,
		cfg.StatusPort, cfg.PDAddrs, cfg.DeployDir, cfg.TmpDir)), &topo)

	if err != nil {
		return nil, err
	}

	conf, err := merge(topo.ServerConfigs.TiFlash, src)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

// InitTiFlashLearnerConfig initializes TiFlash learner config file
func (i *TiFlashInstance) InitTiFlashLearnerConfig(cfg *scripts.TiFlashScript, src map[string]interface{}) (map[string]interface{}, error) {
	topo := TopologySpecification{}

	firstDataDir := strings.Split(cfg.DataDir, ",")[0]

	err := yaml.Unmarshal([]byte(fmt.Sprintf(`
server_configs:
  tiflash-learner:
    log-file: "%[1]s/tiflash_tikv.log"
    server.engine-addr: "%[2]s:%[3]d"
    server.addr: "0.0.0.0:%[4]d"
    server.advertise-addr: "%[2]s:%[4]d"
    server.status-addr: "%[2]s:%[5]d"
    storage.data-dir: "%[6]s/flash"
    rocksdb.wal-dir: ""
    security.ca-path: ""
    security.cert-path: ""
    security.key-path: ""
`, cfg.LogDir, cfg.IP, cfg.FlashServicePort, cfg.FlashProxyPort, cfg.FlashProxyStatusPort, firstDataDir)), &topo)

	if err != nil {
		return nil, err
	}

	conf, err := merge(topo.ServerConfigs.TiFlashLearner, src)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

// InitConfig implement Instance interface
func (i *TiFlashInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(TiFlashSpec)

	tidbStatusAddrs := []string{}
	for _, tidb := range i.instance.topo.TiDBServers {
		tidbStatusAddrs = append(tidbStatusAddrs, fmt.Sprintf("%s:%d", tidb.Host, uint64(tidb.StatusPort)))
	}
	tidbStatusStr := strings.Join(tidbStatusAddrs, ",")

	var pdAddrs []string
	for _, pd := range i.instance.topo.PDServers {
		pdAddrs = append(pdAddrs, fmt.Sprintf("%s:%d", pd.Host, uint64(pd.ClientPort)))
	}
	pdStr := strings.Join(pdAddrs, ",")

	// replication.enable-placement-rules should be set to true to enable TiFlash
	// TODO: Move this logic to an independent checkConfig procedure
	const key = "replication.enable-placement-rules"
	globalEnabled, ok1 := i.instance.topo.ServerConfigs.PD[key].(bool)
	for _, pd := range i.instance.topo.PDServers {
		// if instance config exists AND the config is false, throw an error.
		// if instance config does not exist, if global config does not exist OR the config is false, throw an error
		if instanceEnabled, ok2 := pd.Config[key].(bool); (ok2 && !instanceEnabled) || (!ok2 && (!ok1 || !globalEnabled)) {
			log.Warnf("should set replication.enable-placement-rules to true in pd conf to enable TiFlash")
		}
	}

	cfg := scripts.NewTiFlashScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data,
		paths.Log,
		tidbStatusStr,
		pdStr,
	).WithTCPPort(spec.TCPPort).WithHTTPPort(spec.HTTPPort).WithFlashServicePort(spec.FlashServicePort).
		WithFlashProxyPort(spec.FlashProxyPort).WithFlashProxyStatusPort(spec.FlashProxyStatusPort).
		WithStatusPort(spec.StatusPort).WithTmpDir(spec.TmpDir).AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tiflash_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tiflash.sh")

	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	conf, err := i.InitTiFlashLearnerConfig(cfg, i.instance.topo.ServerConfigs.TiFlashLearner)
	if err != nil {
		return err
	}

	specLernerConfig := spec.LearnerConfig
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			"config",
			fmt.Sprintf(
				"%s-learner-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		mergedConfig, err := mergeImported(importConfig, spec.LearnerConfig)
		if err != nil {
			return err
		}
		specLernerConfig = mergedConfig
	}

	err = i.mergeTiFlashLearnerServerConfig(e, conf, specLernerConfig, paths)
	if err != nil {
		return err
	}

	conf, err = i.InitTiFlashConfig(cfg, i.instance.topo.ServerConfigs.TiFlash)
	if err != nil {
		return err
	}

	specConfig := spec.Config
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			"config",
			fmt.Sprintf(
				"%s-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := ioutil.ReadFile(configPath)
		if err != nil {
			return err
		}
		specConfig, err = mergeImported(importConfig, spec.Config)
		if err != nil {
			return err
		}
	}

	return i.mergeServerConfig(e, conf, specConfig, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiFlashInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = b.GetClusterSpecification()
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// MonitorComponent represents Monitor component.
type MonitorComponent struct{ *ClusterSpecification }

// Name implements Component interface.
func (c *MonitorComponent) Name() string {
	return ComponentPrometheus
}

// Instances implements Component interface.
func (c *MonitorComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Monitors))
	for _, s := range c.Monitors {
		ins = append(ins, &MonitorInstance{instance{
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
	instance
}

// InitConfig implement Instance interface
func (i *MonitorInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	// transfer run script
	spec := i.InstanceSpec.(PrometheusSpec)
	cfg := scripts.NewPrometheusScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data,
		paths.Log,
	).WithPort(spec.Port)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_prometheus_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_prometheus.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("tikv_%s.yml", i.GetHost()))
	cfig := config.NewPrometheusConfig(clusterName)
	cfig.AddBlackbox(i.GetHost(), uint64(i.instance.topo.MonitoredOptions.BlackboxExporterPort))
	uniqueHosts := set.NewStringSet()
	for _, pd := range i.instance.topo.PDServers {
		uniqueHosts.Insert(pd.Host)
		cfig.AddPD(pd.Host, uint64(pd.ClientPort))
	}
	for _, kv := range i.instance.topo.TiKVServers {
		uniqueHosts.Insert(kv.Host)
		cfig.AddTiKV(kv.Host, uint64(kv.StatusPort))
	}
	for _, db := range i.instance.topo.TiDBServers {
		uniqueHosts.Insert(db.Host)
		cfig.AddTiDB(db.Host, uint64(db.StatusPort))
	}
	for _, flash := range i.instance.topo.TiFlashServers {
		uniqueHosts.Insert(flash.Host)
		cfig.AddTiFlashLearner(flash.Host, uint64(flash.FlashProxyStatusPort))
		cfig.AddTiFlash(flash.Host, uint64(flash.StatusPort))
	}
	for _, pump := range i.instance.topo.PumpServers {
		uniqueHosts.Insert(pump.Host)
		cfig.AddPump(pump.Host, uint64(pump.Port))
	}
	for _, drainer := range i.instance.topo.Drainers {
		uniqueHosts.Insert(drainer.Host)
		cfig.AddDrainer(drainer.Host, uint64(drainer.Port))
	}
	for _, cdc := range i.instance.topo.CDCServers {
		uniqueHosts.Insert(cdc.Host)
		cfig.AddCDC(cdc.Host, uint64(cdc.Port))
	}
	for _, grafana := range i.instance.topo.Grafana {
		uniqueHosts.Insert(grafana.Host)
		cfig.AddGrafana(grafana.Host, uint64(grafana.Port))
	}
	for host := range uniqueHosts {
		cfig.AddNodeExpoertor(host, uint64(i.instance.topo.MonitoredOptions.NodeExporterPort))
		cfig.AddBlackboxExporter(host, uint64(i.instance.topo.MonitoredOptions.BlackboxExporterPort))
		cfig.AddMonitoredServer(host)
	}

	if err := cfig.ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "prometheus.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *MonitorInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification,
	clusterName string, clusterVersion string, deployUser string, paths DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = b.GetClusterSpecification()
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// GrafanaComponent represents Grafana component.
type GrafanaComponent struct{ *ClusterSpecification }

// Name implements Component interface.
func (c *GrafanaComponent) Name() string {
	return ComponentGrafana
}

// Instances implements Component interface.
func (c *GrafanaComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Grafana))
	for _, s := range c.Grafana {
		ins = append(ins, &GrafanaInstance{
			instance: instance{
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
					return "-"
				},
			},
		})
	}
	return ins
}

// GrafanaInstance represent the grafana instance
type GrafanaInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *GrafanaInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	// transfer run script
	cfg := scripts.NewGrafanaScript(clusterName, paths.Deploy)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_grafana_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_grafana.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("grafana_%s.ini", i.GetHost()))
	if err := config.NewGrafanaConfig(i.GetHost(), paths.Deploy).WithPort(uint64(i.GetPort())).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "grafana.ini")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer dashboard.yml
	fp = filepath.Join(paths.Cache, fmt.Sprintf("dashboard_%s.yml", i.GetHost()))
	if err := config.NewDashboardConfig(clusterName, paths.Deploy).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "dashboard.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer datasource.yml
	if len(i.instance.topo.Monitors) == 0 {
		return errors.New("no prometheus found in topology")
	}
	fp = filepath.Join(paths.Cache, fmt.Sprintf("datasource_%s.yml", i.GetHost()))
	if err := config.NewDatasourceConfig(clusterName, i.instance.topo.Monitors[0].Host).
		WithPort(uint64(i.instance.topo.Monitors[0].Port)).
		ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "datasource.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *GrafanaInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification,
	clusterName string, clusterVersion string, deployUser string, paths DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = b.GetClusterSpecification().Merge(i.instance.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// AlertManagerComponent represents Alertmanager component.
type AlertManagerComponent struct{ *ClusterSpecification }

// Name implements Component interface.
func (c *AlertManagerComponent) Name() string {
	return ComponentAlertManager
}

// Instances implements Component interface.
func (c *AlertManagerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Alertmanager))
	for _, s := range c.Alertmanager {
		ins = append(ins, &AlertManagerInstance{
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.WebPort,
				sshp:         s.SSHPort,
				topo:         c.ClusterSpecification,

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
			},
		})
	}
	return ins
}

// AlertManagerInstance represent the alert manager instance
type AlertManagerInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *AlertManagerInstance) InitConfig(e executor.TiOpsExecutor, clusterName, clusterVersion, deployUser string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	// Transfer start script
	spec := i.InstanceSpec.(AlertManagerSpec)
	cfg := scripts.NewAlertManagerScript(paths.Deploy, paths.Data, paths.Log).
		WithWebPort(spec.WebPort).WithClusterPort(spec.ClusterPort)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_alertmanager_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_alertmanager.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("alertmanager_%s.yml", i.GetHost()))
	if err := config.NewAlertManagerConfig().ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "alertmanager.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *AlertManagerInstance) ScaleConfig(e executor.TiOpsExecutor, b Specification,
	clusterName string, clusterVersion string, deployUser string, paths DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = b.GetClusterSpecification()
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// GetGlobalOptions returns GlobalOptions
func (topo *ClusterSpecification) GetGlobalOptions() GlobalOptions {
	return topo.GlobalOptions
}

// GetMonitoredOptions returns MonitoredOptions
func (topo *ClusterSpecification) GetMonitoredOptions() MonitoredOptions {
	return topo.MonitoredOptions
}

// GetClusterSpecification returns cluster topology
func (topo *ClusterSpecification) GetClusterSpecification() *ClusterSpecification {
	return topo
}

// GetDMSpecification returns dm topology
func (topo *ClusterSpecification) GetDMSpecification() *DMSpecification {
	return nil
}

// ComponentsByStopOrder return component in the order need to stop.
func (topo *ClusterSpecification) ComponentsByStopOrder() (comps []Component) {
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
func (topo *ClusterSpecification) ComponentsByStartOrder() (comps []Component) {
	// "pd", "tikv", "pump", "tidb", "drainer", "prometheus", "grafana", "alertmanager"
	comps = append(comps, &PDComponent{topo})
	comps = append(comps, &TiKVComponent{topo})
	comps = append(comps, &PumpComponent{topo})
	comps = append(comps, &TiDBComponent{topo})
	comps = append(comps, &TiFlashComponent{topo})
	comps = append(comps, &DrainerComponent{topo})
	comps = append(comps, &CDCComponent{topo})
	comps = append(comps, &MonitorComponent{topo})
	comps = append(comps, &GrafanaComponent{topo})
	comps = append(comps, &AlertManagerComponent{topo})
	return
}

// IterComponent iterates all components in component starting order
func (topo *ClusterSpecification) IterComponent(fn func(comp Component)) {
	for _, comp := range topo.ComponentsByStartOrder() {
		fn(comp)
	}
}

// IterInstance iterates all instances in component starting order
func (topo *ClusterSpecification) IterInstance(fn func(instance Instance)) {
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			fn(inst)
		}
	}
}

// IterHost iterates one instance for each host
func (topo *ClusterSpecification) IterHost(fn func(instance Instance)) {
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
func (topo *ClusterSpecification) Endpoints(user string) []*scripts.PDScript {
	var ends []*scripts.PDScript
	for _, spec := range topo.PDServers {
		deployDir := clusterutil.Abs(user, spec.DeployDir)
		// data dir would be empty for components which don't need it
		dataDir := spec.DataDir
		if dataDir != "" {
			clusterutil.Abs(user, dataDir)
		}
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(user, spec.LogDir)

		script := scripts.NewPDScript(
			spec.Name,
			spec.Host,
			deployDir,
			dataDir,
			logDir).
			WithClientPort(spec.ClientPort).
			WithPeerPort(spec.PeerPort)
		ends = append(ends, script)
	}
	return ends
}
