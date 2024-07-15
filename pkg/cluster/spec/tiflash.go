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
	"io"
	"net/http"
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
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
	"gopkg.in/yaml.v2"
)

// TiFlashSpec represents the TiFlash topology specification in topology.yaml
type TiFlashSpec struct {
	Host                 string               `yaml:"host"`
	ManageHost           string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	SSHPort              int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported             bool                 `yaml:"imported,omitempty"`
	Patched              bool                 `yaml:"patched,omitempty"`
	IgnoreExporter       bool                 `yaml:"ignore_exporter,omitempty"`
	TCPPort              int                  `yaml:"tcp_port" default:"9000"`
	HTTPPort             int                  `yaml:"http_port" default:"8123"` // Deprecated since v7.1.0
	FlashServicePort     int                  `yaml:"flash_service_port" default:"3930"`
	FlashProxyPort       int                  `yaml:"flash_proxy_port" default:"20170"`
	FlashProxyStatusPort int                  `yaml:"flash_proxy_status_port" default:"20292"`
	StatusPort           int                  `yaml:"metrics_port" default:"8234"`
	DeployDir            string               `yaml:"deploy_dir,omitempty"`
	DataDir              string               `yaml:"data_dir,omitempty" validate:"data_dir:expandable"`
	LogDir               string               `yaml:"log_dir,omitempty"`
	TmpDir               string               `yaml:"tmp_path,omitempty"`
	Offline              bool                 `yaml:"offline,omitempty"`
	Source               string               `yaml:"source,omitempty" validate:"source:editable"`
	NumaNode             string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	NumaCores            string               `yaml:"numa_cores,omitempty" validate:"numa_cores:editable"`
	Config               map[string]any       `yaml:"config,omitempty" validate:"config:ignore"`
	LearnerConfig        map[string]any       `yaml:"learner_config,omitempty" validate:"learner_config:ignore"`
	ResourceControl      meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch                 string               `yaml:"arch,omitempty"`
	OS                   string               `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s *TiFlashSpec) Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string {
	storeAddr := utils.JoinHostPort(s.Host, s.FlashServicePort)
	state := checkStoreStatus(ctx, storeAddr, tlsCfg, pdList...)
	if s.Offline && strings.ToLower(state) == "offline" {
		state = "Pending Offline" // avoid misleading
	}
	return state
}

const (
	// EngineLabelKey is the label that indicates the backend of store instance:
	// tikv or tiflash. TiFlash instance will contain a label of 'engine: tiflash'.
	EngineLabelKey = "engine"
	// EngineLabelTiFlash is the label value, which a TiFlash instance will have with
	// a label key of EngineLabelKey.
	EngineLabelTiFlash = "tiflash"
	// EngineLabelTiFlashCompute is for disaggregated tiflash mode,
	// it's the lable of tiflash_compute nodes.
	EngineLabelTiFlashCompute = "tiflash_compute"
	// EngineRoleLabelKey is the label that indicates if the TiFlash instance is a write node.
	EngineRoleLabelKey = "engine_role"
	// EngineRoleLabelWrite is for disaggregated tiflash write node.
	EngineRoleLabelWrite = "write"
)

// GetExtendedRole get extended name for TiFlash to distinguish disaggregated mode.
func (s *TiFlashSpec) GetExtendedRole(ctx context.Context, tlsCfg *tls.Config, pdList ...string) string {
	if len(pdList) < 1 {
		return ""
	}
	storeAddr := utils.JoinHostPort(s.Host, s.FlashServicePort)
	pdapi := api.NewPDClient(ctx, pdList, statusQueryTimeout, tlsCfg)
	store, err := pdapi.GetCurrentStore(storeAddr)
	if err != nil {
		return ""
	}
	isWriteNode := false
	isTiFlash := false
	for _, label := range store.Store.Labels {
		if label.Key == EngineLabelKey {
			if label.Value == EngineLabelTiFlashCompute {
				return " (compute)"
			}
			if label.Value == EngineLabelTiFlash {
				isTiFlash = true
			}
		}
		if label.Key == EngineRoleLabelKey && label.Value == EngineRoleLabelWrite {
			isWriteNode = true
		}
		if isTiFlash && isWriteNode {
			return " (write)"
		}
	}
	return ""
}

// Role returns the component role of the instance
func (s *TiFlashSpec) Role() string {
	return ComponentTiFlash
}

// SSH returns the host and SSH port of the instance
func (s *TiFlashSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiFlashSpec) GetMainPort() int {
	return s.TCPPort
}

// GetManageHost returns the manage host of the instance
func (s *TiFlashSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiFlashSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiFlashSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// key names for storage config
const (
	TiFlashStorageKeyMainDirs   string = "storage.main.dir"
	TiFlashStorageKeyLatestDirs string = "storage.latest.dir"
	TiFlashStorageKeyRaftDirs   string = "storage.raft.dir"
	TiFlashRemoteCacheDir       string = "storage.remote.cache.dir"
	TiFlashRequiredCPUFlags     string = "avx2 popcnt movbe"
)

// GetOverrideDataDir returns the data dir.
// If users have defined TiFlashStorageKeyMainDirs, then override "DataDir" with
// the directories defined in TiFlashStorageKeyMainDirs and TiFlashStorageKeyLatestDirs
func (s *TiFlashSpec) GetOverrideDataDir() (string, error) {
	getStrings := func(key string) []string {
		var strs []string
		if dirsVal, ok := s.Config[key]; ok {
			if dirs, ok := dirsVal.([]any); ok && len(dirs) > 0 {
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
			return fmt.Errorf("directory '%s' should be an absolute path in 'tiflash_servers:%s.config.%s'", d, host, key)
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

// Source implements Component interface.
func (c *TiFlashComponent) Source() string {
	source := c.Topology.ComponentSources.TiFlash
	if source != "" {
		return source
	}
	return ComponentTiFlash
}

// CalculateVersion implements the Component interface
func (c *TiFlashComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.TiFlash
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *TiFlashComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.TiFlash = version
}

// Instances implements Component interface.
func (c *TiFlashComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiFlashServers))
	for _, s := range c.Topology.TiFlashServers {
		tiflashInstance := &TiFlashInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			ManageHost:   s.ManageHost,
			ListenHost:   c.Topology.BaseTopo().GlobalOptions.ListenHost,
			Port:         s.GetMainPort(),
			SSHP:         s.SSHPort,
			Source:       s.Source,
			NumaNode:     s.NumaNode,
			NumaCores:    s.NumaCores,

			Ports: []int{
				s.TCPPort,
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
			UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.GetManageHost(), s.StatusPort, timeout, tlsCfg)
			},
			Component: c,
		}, c.Topology}
		// For 7.1.0 or later, TiFlash HTTP service is removed, so we don't need to set http_port
		if !tidbver.TiFlashNotNeedHTTPPortConfig(c.Topology.ComponentVersions.TiFlash) {
			tiflashInstance.Ports = append(tiflashInstance.Ports, s.HTTPPort)
		}
		ins = append(ins, tiflashInstance)
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

// GetStatusPort returns the status port of TiFlash
func (i *TiFlashInstance) GetStatusPort() int {
	return i.InstanceSpec.(*TiFlashSpec).FlashProxyStatusPort
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
func isValidStringArray(key string, config map[string]any, couldEmpty bool) (bool, error) {
	var (
		dirsVal          any
		isKeyDefined     bool
		isAllElemsString = true
	)
	if dirsVal, isKeyDefined = config[key]; !isKeyDefined {
		return isKeyDefined, nil
	}
	if dirs, ok := dirsVal.([]any); ok && (couldEmpty || len(dirs) > 0) {
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

// checkTiFlashStorageConfig ensures `storage.main` is defined when
// `storage.latest` or `storage.raft` is used.
func checkTiFlashStorageConfig(config map[string]any) (bool, error) {
	isMainStorageDefined, err := isValidStringArray(TiFlashStorageKeyMainDirs, config, false)
	if err != nil {
		return false, err
	}
	if !isMainStorageDefined {
		for k := range config {
			if strings.HasPrefix(k, "storage.latest") || strings.HasPrefix(k, "storage.raft") {
				return false, fmt.Errorf("you must set '%s' before setting '%s', please check the tiflash configuration in your yaml file", TiFlashStorageKeyMainDirs, k)
			}
		}
	}
	return isMainStorageDefined, nil
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
func checkTiFlashStorageConfigWithVersion(clusterVersion string, config map[string]any) (bool, error) {
	if tidbver.TiFlashSupportMultiDisksDeployment(clusterVersion) {
		return checkTiFlashStorageConfig(config)
	}
	return false, nil
}

// InitTiFlashConfig initializes TiFlash config file with the configurations in server_configs
func (i *TiFlashInstance) initTiFlashConfig(ctx context.Context, version string, src map[string]any, paths meta.DirPaths) (map[string]any, error) {
	var (
		pathConfig            string
		isStorageDirsDefined  bool
		deprecatedUsersConfig string
		daemonConfig          string
		markCacheSize         string
		err                   error
	)
	if isStorageDirsDefined, err = checkTiFlashStorageConfigWithVersion(version, src); err != nil {
		return nil, err
	}
	// For backward compatibility, we need to rollback to set 'path'
	if isStorageDirsDefined {
		pathConfig = "#"
	} else {
		pathConfig = fmt.Sprintf(`path: "%s"`, strings.Join(paths.Data, ","))
	}

	if tidbver.TiFlashDeprecatedUsersConfig(version) {
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

	tidbStatusAddrs := []string{}
	for _, tidb := range i.topo.(*Specification).TiDBServers {
		tidbStatusAddrs = append(tidbStatusAddrs, utils.JoinHostPort(tidb.Host, tidb.StatusPort))
	}

	spec := i.InstanceSpec.(*TiFlashSpec)
	enableTLS := i.topo.(*Specification).GlobalOptions.TLSEnabled
	httpPort := "#"
	// For 7.1.0 or later, TiFlash HTTP service is removed, so we don't need to set http_port
	if !tidbver.TiFlashNotNeedHTTPPortConfig(version) {
		if enableTLS {
			httpPort = fmt.Sprintf(`https_port: %d`, spec.HTTPPort)
		} else {
			httpPort = fmt.Sprintf(`http_port: %d`, spec.HTTPPort)
		}
	}
	tcpPort := "#"
	// Config tcp_port is only required for TiFlash version < 7.1.0, and is recommended to not specify for TiFlash version >= 7.1.0.
	if tidbver.TiFlashRequiresTCPPortConfig(version) {
		tcpPort = fmt.Sprintf(`tcp_port: %d`, spec.TCPPort)
	}

	// set TLS configs
	spec.Config, err = i.setTLSConfig(ctx, enableTLS, spec.Config, paths)
	if err != nil {
		return nil, err
	}

	topo := Specification{}

	if tidbver.TiFlashNotNeedSomeConfig(version) {
		// For 5.4.0 or later, TiFlash can ignore application.runAsDaemon and mark_cache_size setting
		daemonConfig = "#"
		markCacheSize = "#"
	} else {
		daemonConfig = `application.runAsDaemon: true`
		markCacheSize = `mark_cache_size: 5368709120`
	}

	err = yaml.Unmarshal([]byte(fmt.Sprintf(`
server_configs:
  tiflash:
    default_profile: "default"
    display_name: "TiFlash"
    listen_host: "%[7]s"
    tmp_path: "%[11]s"
    %[1]s
    %[3]s
    %[4]s
    flash.tidb_status_addr: "%[5]s"
    flash.service_addr: "%[6]s"
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
    logger.size: "1000M"
    %[13]s
    raft.pd_addr: "%[9]s"
    %[12]s
    %[14]s
`,
		pathConfig,
		paths.Log,
		tcpPort,
		httpPort,
		strings.Join(tidbStatusAddrs, ","),
		utils.JoinHostPort(spec.Host, spec.FlashServicePort),
		i.GetListenHost(),
		spec.StatusPort,
		strings.Join(i.topo.(*Specification).GetPDList(), ","),
		paths.Deploy,
		fmt.Sprintf("%s/tmp", paths.Data[0]),
		deprecatedUsersConfig,
		daemonConfig,
		markCacheSize,
	)), &topo)

	if err != nil {
		return nil, err
	}

	conf := MergeConfig(topo.ServerConfigs.TiFlash, spec.Config, src)
	return conf, nil
}

func (i *TiFlashInstance) mergeTiFlashInstanceConfig(clusterVersion string, globalConf, instanceConf map[string]any) (map[string]any, error) {
	var (
		isStorageDirsDefined bool
		err                  error
		conf                 map[string]any
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
func (i *TiFlashInstance) InitTiFlashLearnerConfig(ctx context.Context, clusterVersion string, src map[string]any, paths meta.DirPaths) (map[string]any, error) {
	spec := i.InstanceSpec.(*TiFlashSpec)
	topo := Specification{}
	var statusAddr string

	if tidbver.TiFlashSupportAdvertiseStatusAddr(clusterVersion) {
		statusAddr = fmt.Sprintf(`server.status-addr: "%s"
    server.advertise-status-addr: "%s"`,
			utils.JoinHostPort(i.GetListenHost(), spec.FlashProxyStatusPort),
			utils.JoinHostPort(spec.Host, spec.FlashProxyStatusPort))
	} else {
		statusAddr = fmt.Sprintf(`server.status-addr: "%s"`, utils.JoinHostPort(spec.Host, spec.FlashProxyStatusPort))
	}
	err := yaml.Unmarshal([]byte(fmt.Sprintf(`
server_configs:
  tiflash-learner:
    log-file: "%[1]s/tiflash_tikv.log"
    server.engine-addr: "%[2]s"
    server.addr: "%[3]s"
    server.advertise-addr: "%[4]s"
    %[5]s
    storage.data-dir: "%[6]s/flash"
    rocksdb.wal-dir: ""
    security.ca-path: ""
    security.cert-path: ""
    security.key-path: ""
    # Normally the number of TiFlash nodes is smaller than TiKV nodes, and we need more raft threads to match the write speed of TiKV.
    raftstore.apply-pool-size: 4
    raftstore.store-pool-size: 4
`,
		paths.Log,
		utils.JoinHostPort(spec.Host, spec.FlashServicePort),
		utils.JoinHostPort(i.GetListenHost(), spec.FlashProxyPort),
		utils.JoinHostPort(spec.Host, spec.FlashProxyPort),
		statusAddr,
		paths.Data[0],
	)), &topo)

	if err != nil {
		return nil, err
	}

	enableTLS := i.topo.(*Specification).GlobalOptions.TLSEnabled
	// set TLS configs
	spec.LearnerConfig, err = i.setTLSConfigWithTiFlashLearner(enableTLS, spec.LearnerConfig, paths)
	if err != nil {
		return nil, err
	}

	conf := MergeConfig(topo.ServerConfigs.TiFlashLearner, spec.LearnerConfig, src)
	return conf, nil
}

// setTLSConfigWithTiFlashLearner set TLS Config to support enable/disable TLS
func (i *TiFlashInstance) setTLSConfigWithTiFlashLearner(enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	if enableTLS {
		if configs == nil {
			configs = make(map[string]any)
		}
		configs["security.ca-path"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		configs["security.cert-path"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		configs["security.key-path"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	} else {
		// drainer tls config list
		tlsConfigs := []string{
			"security.ca-path",
			"security.cert-path",
			"security.key-path",
		}
		// delete TLS configs
		if configs != nil {
			for _, config := range tlsConfigs {
				delete(configs, config)
			}
		}
	}

	return configs, nil
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *TiFlashInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	if enableTLS {
		if configs == nil {
			configs = make(map[string]any)
		}
		configs["security.ca_path"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		configs["security.cert_path"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		configs["security.key_path"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	} else {
		// drainer tls config list
		tlsConfigs := []string{
			"security.ca_path",
			"security.cert_path",
			"security.key_path",
		}
		// delete TLS configs
		if configs != nil {
			for _, config := range tlsConfigs {
				delete(configs, config)
			}
		}
	}

	return configs, nil
}

// getTiFlashRequiredCPUFlagsWithVersion return required CPU flags for TiFlash by given version
func getTiFlashRequiredCPUFlagsWithVersion(clusterVersion string, arch string) string {
	arch = strings.ToLower(arch)
	if arch == "x86_64" || arch == "amd64" {
		if tidbver.TiFlashRequireCPUFlagAVX2(clusterVersion) {
			return TiFlashRequiredCPUFlags
		}
	}
	return ""
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
	version := i.CalculateVersion(clusterVersion)

	cfg := &scripts.TiFlashScript{
		RequiredCPUFlags: getTiFlashRequiredCPUFlagsWithVersion(version, spec.Arch),

		DeployDir: paths.Deploy,
		LogDir:    paths.Log,

		NumaNode:  spec.NumaNode,
		NumaCores: spec.NumaCores,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tiflash_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tiflash.sh")

	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	conf, err := i.InitTiFlashLearnerConfig(ctx, version, topo.ServerConfigs.TiFlashLearner, paths)
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
	if conf, err = i.initTiFlashConfig(ctx, version, topo.ServerConfigs.TiFlash, paths); err != nil {
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
	if conf, err = i.mergeTiFlashInstanceConfig(version, conf, spec.Config); err != nil {
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

	endpoints := topo.(*Specification).GetPDListWithManageHost()
	pdClient := api.NewPDClient(ctx, endpoints, 10*time.Second, tlsCfg)
	return pdClient.UpdateReplicateConfig(bytes.NewBuffer(enablePlacementRules))
}

// Ready implements Instance interface
func (i *TiFlashInstance) Ready(ctx context.Context, e ctxt.Executor, timeout uint64, tlsCfg *tls.Config) error {
	// FIXME: the timeout is applied twice in the whole `Ready()` process, in the worst
	// case it might wait double time as other components
	if err := PortStarted(ctx, e, i.GetServicePort(), timeout); err != nil {
		return err
	}

	scheme := "http"
	if i.topo.BaseTopo().GlobalOptions.TLSEnabled {
		scheme = "https"
	}
	addr := fmt.Sprintf("%s://%s/tiflash/store-status", scheme, utils.JoinHostPort(i.GetManageHost(), i.GetStatusPort()))
	req, err := http.NewRequest("GET", addr, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	retryOpt := utils.RetryOption{
		Delay:   time.Second,
		Timeout: time.Second * time.Duration(timeout),
	}
	var queryErr error
	if err := utils.Retry(func() error {
		client := utils.NewHTTPClient(statusQueryTimeout, tlsCfg)
		res, err := client.Client().Do(req)
		if err != nil {
			queryErr = err
			return err
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			queryErr = err
			return err
		}
		if res.StatusCode == http.StatusNotFound || string(body) == "Running" {
			return nil
		}

		err = fmt.Errorf("tiflash store status is '%s', not fully running yet", string(body))
		queryErr = err
		return err
	}, retryOpt); err != nil {
		return perrs.Annotatef(queryErr, "timed out waiting for tiflash %s:%d to be ready after %ds",
			i.Host, i.Port, timeout)
	}
	return nil
}
