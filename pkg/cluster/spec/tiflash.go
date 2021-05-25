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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"
)

// TiFlashSpec represents the TiFlash topology specification in topology.yaml
type TiFlashSpec struct {
	Host                 string                 `yaml:"host"`
	SSHPort              int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported             bool                   `yaml:"imported,omitempty"`
	Patched              bool                   `yaml:"patched,omitempty"`
	TCPPort              int                    `yaml:"tcp_port" default:"9000"`
	HTTPPort             int                    `yaml:"http_port" default:"8123"`
	FlashServicePort     int                    `yaml:"flash_service_port" default:"3930"`
	FlashProxyPort       int                    `yaml:"flash_proxy_port" default:"20170"`
	FlashProxyStatusPort int                    `yaml:"flash_proxy_status_port" default:"20292"`
	StatusPort           int                    `yaml:"metrics_port" default:"8234"`
	DeployDir            string                 `yaml:"deploy_dir,omitempty"`
	DataDir              string                 `yaml:"data_dir,omitempty" validate:"data_dir:expandable"`
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
func (s *TiFlashSpec) Status(tlsCfg *tls.Config, pdList ...string) string {
	storeAddr := fmt.Sprintf("%s:%d", s.Host, s.FlashServicePort)
	state := checkStoreStatus(storeAddr, tlsCfg, pdList...)
	if s.Offline && strings.ToLower(state) == "offline" {
		state = "Pending Offline" // avoid misleading
	}
	return state
}

// Role returns the component role of the instance
func (s *TiFlashSpec) Role() string {
	return ComponentTiFlash
}

// SSH returns the host and SSH port of the instance
func (s *TiFlashSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiFlashSpec) GetMainPort() int {
	return s.TCPPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiFlashSpec) IsImported() bool {
	return s.Imported
}

// key names for storage config
const (
	TiFlashStorageKeyMainDirs   string = "storage.main.dir"
	TiFlashStorageKeyLatestDirs string = "storage.latest.dir"
	TiFlashStorageKeyRaftDirs   string = "storage.raft.dir"
)

// GetOverrideDataDir returns the data dir.
// If users have defined TiFlashStorageKeyMainDirs, then override "DataDir" with
// the directories defined in TiFlashStorageKeyMainDirs and TiFlashStorageKeyLatestDirs
func (s *TiFlashSpec) GetOverrideDataDir() (string, error) {
	getStrings := func(key string) []string {
		var strs []string
		if dirsVal, ok := s.Config[key]; ok {
			if dirs, ok := dirsVal.([]interface{}); ok && len(dirs) > 0 {
				for _, elem := range dirs {
					if elemStr, ok := elem.(string); ok {
						elemStr := strings.TrimSuffix(strings.TrimSpace(elemStr), "/")
						strs = append(strs, elemStr)
					}
				}
			}
		}
		return strs
	}
	mainDirs := getStrings(TiFlashStorageKeyMainDirs)
	latestDirs := getStrings(TiFlashStorageKeyLatestDirs)
	if len(mainDirs) == 0 && len(latestDirs) == 0 {
		return s.DataDir, nil
	}

	// If storage is defined, the path defined in "data_dir" will be ignored
	// check whether the directories is uniq in the same configuration item
	// and make the dirSet uniq
	checkAbsolute := func(d, host, key string) error {
		if !strings.HasPrefix(d, "/") {
			return fmt.Errorf("directory '%s' should be an absolute path in 'tiflash_servers:%s.config.%s'", d, s.Host, key)
		}
		return nil
	}

	dirSet := set.NewStringSet()
	for _, d := range latestDirs {
		if err := checkAbsolute(d, s.Host, TiFlashStorageKeyLatestDirs); err != nil {
			return "", err
		}
		if dirSet.Exist(d) {
			return "", &meta.ValidateErr{
				Type:   meta.TypeConflict,
				Target: "directory",
				LHS:    fmt.Sprintf("tiflash_servers:%s.config.%s", s.Host, TiFlashStorageKeyLatestDirs),
				RHS:    fmt.Sprintf("tiflash_servers:%s.config.%s", s.Host, TiFlashStorageKeyLatestDirs),
				Value:  d,
			}
		}
		dirSet.Insert(d)
	}
	mainDirSet := set.NewStringSet()
	for _, d := range mainDirs {
		if err := checkAbsolute(d, s.Host, TiFlashStorageKeyMainDirs); err != nil {
			return "", err
		}
		if mainDirSet.Exist(d) {
			return "", &meta.ValidateErr{
				Type:   meta.TypeConflict,
				Target: "directory",
				LHS:    fmt.Sprintf("tiflash_servers:%s.config.%s", s.Host, TiFlashStorageKeyMainDirs),
				RHS:    fmt.Sprintf("tiflash_servers:%s.config.%s", s.Host, TiFlashStorageKeyMainDirs),
				Value:  d,
			}
		}
		mainDirSet.Insert(d)
		dirSet.Insert(d)
	}
	// keep the firstPath
	var firstPath string
	if len(latestDirs) != 0 {
		firstPath = latestDirs[0]
	} else {
		firstPath = mainDirs[0]
	}
	dirSet.Remove(firstPath)
	// join (stable sorted) paths with ","
	keys := make([]string, len(dirSet))
	i := 0
	for k := range dirSet {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	joinedPaths := firstPath
	if len(keys) > 0 {
		joinedPaths += "," + strings.Join(keys, ",")
	}
	return joinedPaths, nil
}

// TiFlashComponent represents TiFlash component.
type TiFlashComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiFlashComponent) Name() string {
	return ComponentTiFlash
}

// Role implements Component interface.
func (c *TiFlashComponent) Role() string {
	return ComponentTiFlash
}

// Instances implements Component interface.
func (c *TiFlashComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiFlashServers))
	for _, s := range c.Topology.TiFlashServers {
		ins = append(ins, &TiFlashInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			Port:         s.GetMainPort(),
			SSHP:         s.SSHPort,

			Ports: []int{
				s.TCPPort,
				s.HTTPPort,
				s.FlashServicePort,
				s.FlashProxyPort,
				s.FlashProxyStatusPort,
				s.StatusPort,
			},
			Dirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			StatusFn: s.Status,
			UptimeFn: func(tlsCfg *tls.Config) time.Duration {
				return 0
			},
		}, c.Topology})
	}
	return ins
}

// TiFlashInstance represent the TiFlash instance
type TiFlashInstance struct {
	BaseInstance
	topo Topology
}

// GetServicePort returns the service port of TiFlash
func (i *TiFlashInstance) GetServicePort() int {
	return i.InstanceSpec.(*TiFlashSpec).FlashServicePort
}

// checkIncorrectKey checks TiFlash's key should not be set in config
func (i *TiFlashInstance) checkIncorrectKey(key string) error {
	errMsg := "NOTE: TiFlash `%s` should NOT be set in topo's \"%s\" config, its value will be ignored, you should set `data_dir` in each host instead, please check your topology"
	if dir, ok := i.InstanceSpec.(*TiFlashSpec).Config[key].(string); ok && dir != "" {
		return fmt.Errorf(errMsg, key, "host")
	}
	if dir, ok := i.topo.(*Specification).ServerConfigs.TiFlash[key].(string); ok && dir != "" {
		return fmt.Errorf(errMsg, key, "server_configs")
	}
	return nil
}

// checkIncorrectServerConfigs checks TiFlash's key should not be set in server_config
func (i *TiFlashInstance) checkIncorrectServerConfigs(key string) error {
	errMsg := "NOTE: TiFlash `%[1]s` should NOT be set in topo's \"%[2]s\" config, you should set `%[1]s` in each host instead, please check your topology"
	if _, ok := i.topo.(*Specification).ServerConfigs.TiFlash[key]; ok {
		return fmt.Errorf(errMsg, key, "server_configs")
	}
	return nil
}

// isValidStringArray detect the key in `config` is valid or not.
// The configuration is valid only the key-value is defined, and the
// value is a non-empty string array.
// Return (key is defined or not, the value is valid or not)
func isValidStringArray(key string, config map[string]interface{}, couldEmpty bool) (bool, error) {
	var (
		dirsVal          interface{}
		isKeyDefined     bool
		isAllElemsString bool = true
	)
	if dirsVal, isKeyDefined = config[key]; !isKeyDefined {
		return isKeyDefined, nil
	}
	if dirs, ok := dirsVal.([]interface{}); ok && (couldEmpty || len(dirs) > 0) {
		// ensure dirs is non-empty string array
		for _, elem := range dirs {
			if _, ok := elem.(string); !ok {
				isAllElemsString = false
				break
			}
		}
		if isAllElemsString {
			return isKeyDefined, nil
		}
	}
	return isKeyDefined, fmt.Errorf("'%s' should be a non-empty string array, please check the tiflash configuration in your yaml file", TiFlashStorageKeyMainDirs)
}

// checkTiFlashStorageConfig detect the "storage" section in `config`
// is valid or not.
func checkTiFlashStorageConfig(config map[string]interface{}) (bool, error) {
	var (
		isStorageDirsDefined bool
		err                  error
	)
	if isStorageDirsDefined, err = isValidStringArray(TiFlashStorageKeyMainDirs, config, false); err != nil {
		return isStorageDirsDefined, err
	}
	if !isStorageDirsDefined {
		containsStorageSectionKey := func(config map[string]interface{}) (string, bool) {
			for k := range config {
				if strings.HasPrefix(k, "storage.") {
					return k, true
				}
			}
			return "", false
		}
		if key, contains := containsStorageSectionKey(config); contains {
			return isStorageDirsDefined, fmt.Errorf("You must set '%s' before setting '%s', please check the tiflash configuration in your yaml file", TiFlashStorageKeyMainDirs, key)
		}
	}
	return isStorageDirsDefined, nil
}

// CheckIncorrectConfigs checks incorrect settings
func (i *TiFlashInstance) CheckIncorrectConfigs() error {
	// data_dir / path should not be set in config
	if err := i.checkIncorrectKey("data_dir"); err != nil {
		return err
	}
	if err := i.checkIncorrectKey("path"); err != nil {
		return err
	}
	// storage.main/latest/raft.dir should not be set in server_config
	if err := i.checkIncorrectServerConfigs(TiFlashStorageKeyMainDirs); err != nil {
		return err
	}
	if err := i.checkIncorrectServerConfigs(TiFlashStorageKeyLatestDirs); err != nil {
		return err
	}
	if err := i.checkIncorrectServerConfigs(TiFlashStorageKeyRaftDirs); err != nil {
		return err
	}
	// storage.* in instance level
	if _, err := checkTiFlashStorageConfig(i.InstanceSpec.(*TiFlashSpec).Config); err != nil {
		return err
	}
	// no matter storage.latest.dir is defined or not, return err
	_, err := isValidStringArray(TiFlashStorageKeyLatestDirs, i.InstanceSpec.(*TiFlashSpec).Config, true)
	return err
}

// need to check the configuration after clusterVersion >= v4.0.9.
func checkTiFlashStorageConfigWithVersion(clusterVersion string, config map[string]interface{}) (bool, error) {
	if semver.Compare(clusterVersion, "v4.0.9") >= 0 || utils.Version(clusterVersion).IsNightly() {
		return checkTiFlashStorageConfig(config)
	}
	return false, nil
}

// InitTiFlashConfig initializes TiFlash config file with the configurations in server_configs
func (i *TiFlashInstance) initTiFlashConfig(cfg *scripts.TiFlashScript, clusterVersion string, src map[string]interface{}) (map[string]interface{}, error) {
	var (
		pathConfig            string
		isStorageDirsDefined  bool
		deprecatedUsersConfig string
		err                   error
	)
	if isStorageDirsDefined, err = checkTiFlashStorageConfigWithVersion(clusterVersion, src); err != nil {
		return nil, err
	}
	// For backward compatibility, we need to rollback to set 'path'
	if isStorageDirsDefined {
		pathConfig = "#"
	} else {
		pathConfig = fmt.Sprintf(`path: "%s"`, cfg.DataDir)
	}

	if (semver.Compare(clusterVersion, "v4.0.12") >= 0 && semver.Compare(clusterVersion, "v5.0.0-rc") != 0) || utils.Version(clusterVersion).IsNightly() {
		// For v4.0.12 or later, 5.0.0 or later, TiFlash can ignore these `user.*`, `quotas.*` settings
		deprecatedUsersConfig = "#"
	} else {
		// These settings is required when the version is earlier than v4.0.12 and v5.0.0
		deprecatedUsersConfig = `
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
    profiles.default.use_uncompressed_cache: 0
    profiles.readonly.readonly: 1
`
	}

	topo := Specification{}

	err = yaml.Unmarshal([]byte(fmt.Sprintf(`
server_configs:
  tiflash:
    default_profile: "default"
    display_name: "TiFlash"
    listen_host: "0.0.0.0"
    mark_cache_size: 5368709120
    tmp_path: "%[11]s"
    %[1]s
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
    profiles.default.max_memory_usage: 0
    %[12]s
`, pathConfig, cfg.LogDir, cfg.TCPPort, cfg.HTTPPort, cfg.TiDBStatusAddrs, cfg.IP, cfg.FlashServicePort,
		cfg.StatusPort, cfg.PDAddrs, cfg.DeployDir, cfg.TmpDir, deprecatedUsersConfig)), &topo)

	if err != nil {
		return nil, err
	}

	conf := MergeConfig(topo.ServerConfigs.TiFlash, src)
	return conf, nil
}

func (i *TiFlashInstance) mergeTiFlashInstanceConfig(clusterVersion string, globalConf, instanceConf map[string]interface{}) (map[string]interface{}, error) {
	var (
		isStorageDirsDefined bool
		err                  error
		conf                 map[string]interface{}
	)
	if isStorageDirsDefined, err = checkTiFlashStorageConfigWithVersion(clusterVersion, instanceConf); err != nil {
		return nil, err
	}
	if isStorageDirsDefined {
		delete(globalConf, "path")
	}

	conf = MergeConfig(globalConf, instanceConf)
	return conf, nil
}

// InitTiFlashLearnerConfig initializes TiFlash learner config file
func (i *TiFlashInstance) InitTiFlashLearnerConfig(cfg *scripts.TiFlashScript, clusterVersion string, src map[string]interface{}) (map[string]interface{}, error) {
	topo := Specification{}
	var statusAddr string

	firstDataDir := strings.Split(cfg.DataDir, ",")[0]

	if semver.Compare(clusterVersion, "v4.0.5") >= 0 || utils.Version(clusterVersion).IsNightly() {
		statusAddr = fmt.Sprintf(`server.status-addr: "0.0.0.0:%[2]d"
    server.advertise-status-addr: "%[1]s:%[2]d"`, cfg.IP, cfg.FlashProxyStatusPort)
	} else {
		statusAddr = fmt.Sprintf(`server.status-addr: "%[1]s:%[2]d"`, cfg.IP, cfg.FlashProxyStatusPort)
	}
	err := yaml.Unmarshal([]byte(fmt.Sprintf(`
server_configs:
  tiflash-learner:
    log-file: "%[1]s/tiflash_tikv.log"
    server.engine-addr: "%[2]s:%[3]d"
    server.addr: "0.0.0.0:%[4]d"
    server.advertise-addr: "%[2]s:%[4]d"
    %[5]s
    storage.data-dir: "%[6]s/flash"
    rocksdb.wal-dir: ""
    security.ca-path: ""
    security.cert-path: ""
    security.key-path: ""
    # Normally the number of TiFlash nodes is smaller than TiKV nodes, and we need more raft threads to match the write speed of TiKV.
    raftstore.apply-pool-size: 4
    raftstore.store-pool-size: 4
`, cfg.LogDir, cfg.IP, cfg.FlashServicePort, cfg.FlashProxyPort, statusAddr, firstDataDir)), &topo)

	if err != nil {
		return nil, err
	}

	conf := MergeConfig(topo.ServerConfigs.TiFlashLearner, src)
	return conf, nil
}

// InitConfig implement Instance interface
func (i *TiFlashInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	topo := i.topo.(*Specification)
	if err := i.BaseInstance.InitConfig(ctx, e, topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(*TiFlashSpec)

	tidbStatusAddrs := []string{}
	for _, tidb := range topo.TiDBServers {
		tidbStatusAddrs = append(tidbStatusAddrs, fmt.Sprintf("%s:%d", tidb.Host, uint64(tidb.StatusPort)))
	}
	tidbStatusStr := strings.Join(tidbStatusAddrs, ",")

	pdStr := strings.Join(i.getEndpoints(i.topo), ",")

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
		AppendEndpoints(topo.Endpoints(deployUser)...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tiflash_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tiflash.sh")

	if err := e.Transfer(ctx, fp, dst, false, 0); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	conf, err := i.InitTiFlashLearnerConfig(cfg, clusterVersion, topo.ServerConfigs.TiFlashLearner)
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
		importConfig, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		conf, err = mergeImported(importConfig, conf)
		if err != nil {
			return err
		}
	}

	err = i.mergeTiFlashLearnerServerConfig(ctx, e, conf, spec.LearnerConfig, paths)
	if err != nil {
		return err
	}

	// Init the configuration using cfg and server_configs
	if conf, err = i.initTiFlashConfig(cfg, clusterVersion, topo.ServerConfigs.TiFlash); err != nil {
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
		importConfig, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		// TODO: maybe we also need to check the imported config?
		// if _, err = checkTiFlashStorageConfigWithVersion(clusterVersion, importConfig); err != nil {
		// 	return err
		// }
		conf, err = mergeImported(importConfig, conf)
		if err != nil {
			return err
		}
	}

	// Check the configuration of instance level
	if conf, err = i.mergeTiFlashInstanceConfig(clusterVersion, conf, spec.Config); err != nil {
		return err
	}

	return i.MergeServerConfig(ctx, e, conf, nil, paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiFlashInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() {
		i.topo = s
	}()
	i.topo = mustBeClusterTopo(topo)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

type replicateConfig struct {
	EnablePlacementRules string `json:"enable-placement-rules"`
}

func (i *TiFlashInstance) getEndpoints(topo Topology) []string {
	var endpoints []string
	for _, pd := range topo.(*Specification).PDServers {
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", pd.Host, uint64(pd.ClientPort)))
	}
	return endpoints
}

// PrepareStart checks TiFlash requirements before starting
func (i *TiFlashInstance) PrepareStart(ctx context.Context, tlsCfg *tls.Config) error {
	// set enable-placement-rules to true via PDClient
	enablePlacementRules, err := json.Marshal(replicateConfig{
		EnablePlacementRules: "true",
	})
	// this should not failed, else exit
	if err != nil {
		return perrs.Annotate(err, "failed to marshal replicate config")
	}

	var topo Topology
	if topoVal := ctx.Value(ctxt.CtxBaseTopo); topoVal != nil { // in scale-out phase
		var ok bool
		topo, ok = topoVal.(Topology)
		if !ok {
			return perrs.New("base topology in context is invalid")
		}
	} else { // in start phase
		topo = i.topo
	}

	endpoints := i.getEndpoints(topo)
	pdClient := api.NewPDClient(endpoints, 10*time.Second, tlsCfg)
	return pdClient.UpdateReplicateConfig(bytes.NewBuffer(enablePlacementRules))
}
