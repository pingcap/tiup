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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"gopkg.in/yaml.v2"
)

// TiFlashSpec represents the TiFlash topology specification in topology.yaml
type TiFlashSpec struct {
	Host                 string                 `yaml:"host"`
	SSHPort              int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported             bool                   `yaml:"imported,omitempty"`
	TCPPort              int                    `yaml:"tcp_port" default:"9000"`
	HTTPPort             int                    `yaml:"http_port" default:"8123"`
	FlashServicePort     int                    `yaml:"flash_service_port" default:"3930"`
	FlashProxyPort       int                    `yaml:"flash_proxy_port" default:"20170"`
	FlashProxyStatusPort int                    `yaml:"flash_proxy_status_port" default:"20292"`
	StatusPort           int                    `yaml:"metrics_port" default:"8234"`
	DeployDir            string                 `yaml:"deploy_dir,omitempty"`
	DataDir              string                 `yaml:"data_dir,omitempty"`
	LogDir               string                 `yaml:"log_dir,omitempty"`
	TmpDir               string                 `yaml:"tmp_path,omitempty"`
	Offline              bool                   `yaml:"offline,omitempty"`
	NumaNode             string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config               map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	LearnerConfig        map[string]interface{} `yaml:"learner_config,omitempty" validate:"learner_config:editable"`
	ResourceControl      meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch                 string                 `yaml:"arch,omitempty"`
	OS                   string                 `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s TiFlashSpec) Status(pdList ...string) string {
	storeAddr := fmt.Sprintf("%s:%d", s.Host, s.FlashServicePort)
	state := checkStoreStatus(storeAddr, pdList...)
	if s.Offline && strings.ToLower(state) == "offline" {
		state = "Pending Offline" // avoid misleading
	}
	return state
}

// Role returns the component role of the instance
func (s TiFlashSpec) Role() string {
	return ComponentTiFlash
}

// SSH returns the host and SSH port of the instance
func (s TiFlashSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiFlashSpec) GetMainPort() int {
	return s.TCPPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiFlashSpec) IsImported() bool {
	return s.Imported
}

// TiFlashComponent represents TiFlash component.
type TiFlashComponent struct{ *Specification }

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
			topo:         c.Specification,

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

// GetServicePort returns the service port of TiFlash
func (i *TiFlashInstance) GetServicePort() int {
	return i.InstanceSpec.(TiFlashSpec).FlashServicePort
}

// checkIncorrectDataDir checks TiFlash's key should not be set in config
func (i *TiFlashInstance) checkIncorrectKey(key string) error {
	errMsg := "NOTE: TiFlash `%s` is should NOT be set in topo's \"%s\" config, its value will be ignored, you should set `data_dir` in each host instead, please check your topology"
	if dir, ok := i.InstanceSpec.(TiFlashSpec).Config[key].(string); ok && dir != "" {
		return fmt.Errorf(errMsg, key, "host")
	}
	if dir, ok := i.instance.topo.ServerConfigs.TiFlash[key].(string); ok && dir != "" {
		return fmt.Errorf(errMsg, key, "server_configs")
	}
	return nil
}

// checkIncorrectDataDir checks incorrect data_dir settings
func (i *TiFlashInstance) checkIncorrectDataDir() error {
	if err := i.checkIncorrectKey("data_dir"); err != nil {
		return err
	}
	return i.checkIncorrectKey("path")
}

// DataDir represents TiFlash's DataDir
func (i *TiFlashInstance) DataDir() string {
	if err := i.checkIncorrectDataDir(); err != nil {
		log.Errorf(err.Error())
	}
	dataDir := reflect.ValueOf(i.InstanceSpec).FieldByName("DataDir")
	if !dataDir.IsValid() {
		return ""
	}
	var dirs []string
	for _, dir := range strings.Split(dataDir.String(), ",") {
		if dir == "" {
			continue
		}
		if !strings.HasPrefix(dir, "/") {
			dirs = append(dirs, filepath.Join(i.DeployDir(), dir))
		} else {
			dirs = append(dirs, dir)
		}
	}
	return strings.Join(dirs, ",")
}

// InitTiFlashConfig initializes TiFlash config file
func (i *TiFlashInstance) InitTiFlashConfig(cfg *scripts.TiFlashScript, src map[string]interface{}) (map[string]interface{}, error) {
	topo := Specification{}

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
	topo := Specification{}

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
    # Normally the number of TiFlash nodes is smaller than TiKV nodes, and we need more raft threads to match the write speed of TiKV.
    raftstore.apply-pool-size: 4
    raftstore.store-pool-size: 4
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
func (i *TiFlashInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(TiFlashSpec)

	tidbStatusAddrs := []string{}
	for _, tidb := range i.instance.topo.TiDBServers {
		tidbStatusAddrs = append(tidbStatusAddrs, fmt.Sprintf("%s:%d", tidb.Host, uint64(tidb.StatusPort)))
	}
	tidbStatusStr := strings.Join(tidbStatusAddrs, ",")

	pdStr := strings.Join(i.getEndpoints(), ",")

	cfg := scripts.NewTiFlashScript(
		i.GetHost(),
		paths.Deploy,
		strings.Join(paths.Data, ","),
		paths.Log,
		tidbStatusStr,
		pdStr,
	).WithTCPPort(spec.TCPPort).
		WithHTTPPort(spec.HTTPPort).
		WithFlashServicePort(spec.FlashServicePort).
		WithFlashProxyPort(spec.FlashProxyPort).
		WithFlashProxyStatusPort(spec.FlashProxyStatusPort).
		WithStatusPort(spec.StatusPort).
		WithTmpDir(spec.TmpDir).
		WithNumaNode(spec.NumaNode).
		AppendEndpoints(i.instance.topo.Endpoints(deployUser)...)

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

	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			AnsibleImportedConfigPath,
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
		conf, err = mergeImported(importConfig, conf)
		if err != nil {
			return err
		}
	}

	err = i.mergeTiFlashLearnerServerConfig(e, conf, spec.LearnerConfig, paths)
	if err != nil {
		return err
	}

	conf, err = i.InitTiFlashConfig(cfg, i.instance.topo.ServerConfigs.TiFlash)
	if err != nil {
		return err
	}

	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			AnsibleImportedConfigPath,
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
		conf, err = mergeImported(importConfig, conf)
		if err != nil {
			return err
		}
	}

	return i.mergeServerConfig(e, conf, spec.Config, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiFlashInstance) ScaleConfig(e executor.Executor, topo Topology, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = mustBeClusterTopo(topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

type replicateConfig struct {
	EnablePlacementRules string `json:"enable-placement-rules"`
}

func (i *TiFlashInstance) getEndpoints() []string {
	var endpoints []string
	for _, pd := range i.instance.topo.PDServers {
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", pd.Host, uint64(pd.ClientPort)))
	}
	return endpoints
}

// PrepareStart checks TiFlash requirements before starting
func (i *TiFlashInstance) PrepareStart() error {
	endPoints := i.getEndpoints()
	// set enable-placement-rules to true via PDClient
	pdClient := api.NewPDClient(endPoints, 10*time.Second, nil)
	enablePlacementRules, err := json.Marshal(replicateConfig{
		EnablePlacementRules: "true",
	})
	if err != nil {
		return nil
	}

	return pdClient.UpdateReplicateConfig(bytes.NewBuffer(enablePlacementRules))
}
