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
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

const (
	// Timeout in second when quering node status
	statusQueryTimeout = 10 * time.Second

	// the prometheus metric name of start time of the process since unix epoch in seconds.
	promMetricStartTimeSeconds = "process_start_time_seconds"
)

// FullHostType is the type of fullhost operations
type FullHostType string

const (
	// FullArchType cpu-arch type
	FullArchType FullHostType = "Arch"
	// FullOSType kernel-name
	FullOSType FullHostType = "OS"
)

// general role names
var (
	RoleMonitor       = "monitor"
	RoleTiSparkMaster = "tispark-master"
	RoleTiSparkWorker = "tispark-worker"
	TopoTypeTiDB      = "tidb-cluster"
	TopoTypeDM        = "dm-cluster"
)

type (
	// InstanceSpec represent a instance specification
	InstanceSpec interface {
		Role() string
		SSH() (string, int)
		GetMainPort() int
		IsImported() bool
		IgnoreMonitorAgent() bool
	}

	// GlobalOptions represents the global options for all groups in topology
	// specification in topology.yaml
	GlobalOptions struct {
		User            string               `yaml:"user,omitempty" default:"tidb"`
		Group           string               `yaml:"group,omitempty"`
		SSHPort         int                  `yaml:"ssh_port,omitempty" default:"22" validate:"ssh_port:editable"`
		SSHType         executor.SSHType     `yaml:"ssh_type,omitempty" default:"builtin"`
		TLSEnabled      bool                 `yaml:"enable_tls,omitempty"`
		DeployDir       string               `yaml:"deploy_dir,omitempty" default:"deploy"`
		DataDir         string               `yaml:"data_dir,omitempty" default:"data"`
		LogDir          string               `yaml:"log_dir,omitempty"`
		ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
		OS              string               `yaml:"os,omitempty" default:"linux"`
		Arch            string               `yaml:"arch,omitempty"`
		Custom          interface{}          `yaml:"custom,omitempty" validate:"custom:ignore"`
	}

	// MonitoredOptions represents the monitored node configuration
	MonitoredOptions struct {
		NodeExporterPort     int                  `yaml:"node_exporter_port,omitempty" default:"9100"`
		BlackboxExporterPort int                  `yaml:"blackbox_exporter_port,omitempty" default:"9115"`
		DeployDir            string               `yaml:"deploy_dir,omitempty"`
		DataDir              string               `yaml:"data_dir,omitempty"`
		LogDir               string               `yaml:"log_dir,omitempty"`
		NumaNode             string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
		ResourceControl      meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	}

	// ServerConfigs represents the server runtime configuration
	ServerConfigs struct {
		TiDB           map[string]interface{} `yaml:"tidb"`
		TiKV           map[string]interface{} `yaml:"tikv"`
		PD             map[string]interface{} `yaml:"pd"`
		TiFlash        map[string]interface{} `yaml:"tiflash"`
		TiFlashLearner map[string]interface{} `yaml:"tiflash-learner"`
		Pump           map[string]interface{} `yaml:"pump"`
		Drainer        map[string]interface{} `yaml:"drainer"`
		CDC            map[string]interface{} `yaml:"cdc"`
		Grafana        map[string]string      `yaml:"grafana"`
	}

	// Specification represents the specification of topology.yaml
	Specification struct {
		GlobalOptions    GlobalOptions        `yaml:"global,omitempty" validate:"global:editable"`
		MonitoredOptions MonitoredOptions     `yaml:"monitored,omitempty" validate:"monitored:editable"`
		ServerConfigs    ServerConfigs        `yaml:"server_configs,omitempty" validate:"server_configs:ignore"`
		TiDBServers      []*TiDBSpec          `yaml:"tidb_servers"`
		TiKVServers      []*TiKVSpec          `yaml:"tikv_servers"`
		TiFlashServers   []*TiFlashSpec       `yaml:"tiflash_servers"`
		PDServers        []*PDSpec            `yaml:"pd_servers"`
		PumpServers      []*PumpSpec          `yaml:"pump_servers,omitempty"`
		Drainers         []*DrainerSpec       `yaml:"drainer_servers,omitempty"`
		CDCServers       []*CDCSpec           `yaml:"cdc_servers,omitempty"`
		TiSparkMasters   []*TiSparkMasterSpec `yaml:"tispark_masters,omitempty"`
		TiSparkWorkers   []*TiSparkWorkerSpec `yaml:"tispark_workers,omitempty"`
		Monitors         []*PrometheusSpec    `yaml:"monitoring_servers"`
		Grafanas         []*GrafanaSpec       `yaml:"grafana_servers,omitempty"`
		Alertmanagers    []*AlertmanagerSpec  `yaml:"alertmanager_servers,omitempty"`
	}
)

// BaseTopo is the base info to topology.
type BaseTopo struct {
	GlobalOptions    *GlobalOptions
	MonitoredOptions *MonitoredOptions
	MasterList       []string

	Monitors      []*PrometheusSpec
	Grafanas      []*GrafanaSpec
	Alertmanagers []*AlertmanagerSpec
}

// Topology represents specification of the cluster.
type Topology interface {
	Type() string
	BaseTopo() *BaseTopo
	// Validate validates the topology specification and produce error if the
	// specification invalid (e.g: port conflicts or directory conflicts)
	Validate() error

	// Instances() []Instance
	ComponentsByStartOrder() []Component
	ComponentsByStopOrder() []Component
	ComponentsByUpdateOrder() []Component
	IterInstance(fn func(instance Instance))
	GetMonitoredOptions() *MonitoredOptions
	// count how many time a path is used by instances in cluster
	CountDir(host string, dir string) int
	TLSConfig(dir string) (*tls.Config, error)
	Merge(that Topology) Topology
	FillHostArchOrOS(hostArchmap map[string]string, fullType FullHostType) error
	GetGrafanaConfig() map[string]string

	ScaleOutTopology
}

// BaseMeta is the base info of metadata.
type BaseMeta struct {
	User    string
	Group   string
	Version string
	OpsVer  *string `yaml:"last_ops_ver,omitempty"` // the version of ourself that updated the meta last time
}

// Metadata of a cluster.
type Metadata interface {
	GetTopology() Topology
	SetTopology(topo Topology)
	GetBaseMeta() *BaseMeta

	UpgradableMetadata
}

// ScaleOutTopology represents a scale out metadata.
type ScaleOutTopology interface {
	// Inherit existing global configuration. We must assign the inherited values before unmarshalling
	// because some default value rely on the global options and monitored options.
	// TODO: we should separate the  unmarshal and setting default value.
	NewPart() Topology
	MergeTopo(topo Topology) Topology
}

// UpgradableMetadata represents a upgradable Metadata.
type UpgradableMetadata interface {
	SetVersion(s string)
	SetUser(u string)
}

// NewPart implements ScaleOutTopology interface.
func (s *Specification) NewPart() Topology {
	return &Specification{
		GlobalOptions:    s.GlobalOptions,
		MonitoredOptions: s.MonitoredOptions,
		ServerConfigs:    s.ServerConfigs,
	}
}

// MergeTopo implements ScaleOutTopology interface.
func (s *Specification) MergeTopo(topo Topology) Topology {
	other, ok := topo.(*Specification)
	if !ok {
		panic("topo should be Specification")
	}

	return s.Merge(other)
}

// GetMonitoredOptions implements Topology interface.
func (s *Specification) GetMonitoredOptions() *MonitoredOptions {
	return &s.MonitoredOptions
}

// TLSConfig generates a tls.Config for the specification as needed
func (s *Specification) TLSConfig(dir string) (*tls.Config, error) {
	if !s.GlobalOptions.TLSEnabled {
		return nil, nil
	}
	tlsConfig, err := LoadClientCert(dir)
	if err != nil {
		return nil, errorx.EnsureStackTrace(err).
			WithProperty(tui.SuggestionFromString("TLS is enabled, but the TLS configuration cannot be obtained"))
	}

	return tlsConfig, nil
}

// Type implements Topology interface.
func (s *Specification) Type() string {
	return TopoTypeTiDB
}

// BaseTopo implements Topology interface.
func (s *Specification) BaseTopo() *BaseTopo {
	return &BaseTopo{
		GlobalOptions:    &s.GlobalOptions,
		MonitoredOptions: s.GetMonitoredOptions(),
		MasterList:       s.GetPDList(),
		Monitors:         s.Monitors,
		Grafanas:         s.Grafanas,
		Alertmanagers:    s.Alertmanagers,
	}
}

// LocationLabels returns replication.location-labels in PD config
func (s *Specification) LocationLabels() ([]string, error) {
	lbs := []string{}

	// We don't allow user define location-labels in instance config
	for _, pd := range s.PDServers {
		if GetValueFromPath(pd.Config, "replication.location-labels") != nil {
			return nil, errors.Errorf(
				"replication.location-labels can't be defined in instance %s:%d, please move it to the global server_configs field",
				pd.Host,
				pd.GetMainPort(),
			)
		}
	}

	if repLbs := GetValueFromPath(s.ServerConfigs.PD, "replication.location-labels"); repLbs != nil {
		for _, l := range repLbs.([]interface{}) {
			lb, ok := l.(string)
			if !ok {
				return nil, errors.Errorf("replication.location-labels contains non-string label: %v", l)
			}
			lbs = append(lbs, lb)
		}
	}

	return lbs, nil
}

// GetTiKVLabels implements TiKVLabelProvider
func (s *Specification) GetTiKVLabels() (map[string]map[string]string, []map[string]api.LabelInfo, error) {
	kvs := s.TiKVServers
	locationLabels := map[string]map[string]string{}
	for _, kv := range kvs {
		address := fmt.Sprintf("%s:%d", kv.Host, kv.GetMainPort())
		var err error
		if locationLabels[address], err = kv.Labels(); err != nil {
			return nil, nil, err
		}
	}
	return locationLabels, nil, nil
}

// AllComponentNames contains the names of all components.
// should include all components in ComponentsByStartOrder
func AllComponentNames() (roles []string) {
	tp := &Specification{}
	tp.IterComponent(func(c Component) {
		switch c.Name() {
		case ComponentTiSpark: // tispark-{master, worker}
			roles = append(roles, c.Role())
		default:
			roles = append(roles, c.Name())
		}
	})

	return
}

// UnmarshalYAML implements the yaml.Unmarshaler interface,
// it sets the default values when unmarshaling the topology file
func (s *Specification) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type topology Specification
	if err := unmarshal((*topology)(s)); err != nil {
		return err
	}

	// set default values from tag
	if err := defaults.Set(s); err != nil {
		return errors.Trace(err)
	}

	// Set monitored options
	if s.MonitoredOptions.DeployDir == "" {
		s.MonitoredOptions.DeployDir = filepath.Join(s.GlobalOptions.DeployDir,
			fmt.Sprintf("%s-%d", RoleMonitor, s.MonitoredOptions.NodeExporterPort))
	}
	if s.MonitoredOptions.DataDir == "" {
		s.MonitoredOptions.DataDir = filepath.Join(s.GlobalOptions.DataDir,
			fmt.Sprintf("%s-%d", RoleMonitor, s.MonitoredOptions.NodeExporterPort))
	}
	if s.MonitoredOptions.LogDir == "" {
		s.MonitoredOptions.LogDir = "log"
	}
	if !strings.HasPrefix(s.MonitoredOptions.LogDir, "/") &&
		!strings.HasPrefix(s.MonitoredOptions.LogDir, s.MonitoredOptions.DeployDir) {
		s.MonitoredOptions.LogDir = filepath.Join(s.MonitoredOptions.DeployDir, s.MonitoredOptions.LogDir)
	}

	// populate custom default values as needed
	if err := fillCustomDefaults(&s.GlobalOptions, s); err != nil {
		return err
	}

	// Rewrite TiFlashSpec.DataDir since we may override it with configurations.
	// Should do it before validatation because we need to detect dir conflicts.
	for i := 0; i < len(s.TiFlashServers); i++ {
		dataDir, err := s.TiFlashServers[i].GetOverrideDataDir()
		if err != nil {
			return err
		}
		if s.TiFlashServers[i].DataDir != dataDir {
			zap.L().Info(
				"tiflash data dir is overwritten by its storage configuration",
				zap.String("host", s.TiFlashServers[i].Host),
				zap.String("dir", dataDir),
			)
			s.TiFlashServers[i].DataDir = dataDir
		}
	}

	// --initial-commit-ts should not be recorded at run_drainer.sh #1682
	s.removeCommitTS()

	return s.Validate()
}

func findField(v reflect.Value, fieldName string) (int, bool) {
	for i := 0; i < reflect.Indirect(v).NumField(); i++ {
		if reflect.Indirect(v).Type().Field(i).Name == fieldName {
			return i, true
		}
	}
	return -1, false
}

func findSliceField(v Topology, fieldName string) (reflect.Value, bool) {
	topo := reflect.ValueOf(v)
	if topo.Kind() == reflect.Ptr {
		topo = topo.Elem()
	}

	j, found := findField(topo, fieldName)
	if found {
		val := topo.Field(j)
		if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
			return val, true
		}
	}
	return reflect.Value{}, false
}

// GetPDList returns a list of PD API hosts of the current cluster
func (s *Specification) GetPDList() []string {
	var pdList []string

	for _, pd := range s.PDServers {
		pdList = append(pdList, fmt.Sprintf("%s:%d", pd.Host, pd.ClientPort))
	}

	return pdList
}

// AdjustByVersion modify the spec by cluster version.
func (s *Specification) AdjustByVersion(clusterVersion string) {
	// CDC does not support data dir for version below v4.0.13, and also v5.0.0-rc, set it to empty.
	if semver.Compare(clusterVersion, "v4.0.13") == -1 || clusterVersion == "v5.0.0-rc" {
		for _, server := range s.CDCServers {
			server.DataDir = ""
		}
	}
	if semver.Compare(clusterVersion, "v5.4.0") >= 0 || strings.Contains(clusterVersion, utils.NightlyVersionAlias) {
		for _, m := range s.Monitors {
			if m.NgPort == 0 {
				m.NgPort = 12020
			}
		}
	}
}

// GetDashboardAddress returns the cluster's dashboard addr
func (s *Specification) GetDashboardAddress(ctx context.Context, tlsCfg *tls.Config, pdList ...string) (string, error) {
	pc := api.NewPDClient(ctx, pdList, statusQueryTimeout, tlsCfg)
	dashboardAddr, err := pc.GetDashboardAddress()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(dashboardAddr, "http") {
		r := strings.NewReplacer("http://", "", "https://", "")
		dashboardAddr = r.Replace(dashboardAddr)
	}
	return dashboardAddr, nil
}

// GetEtcdClient loads EtcdClient of current cluster
func (s *Specification) GetEtcdClient(tlsCfg *tls.Config) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints: s.GetPDList(),
		TLS:       tlsCfg,
	})
}

// GetEtcdProxyClient loads EtcdClient of current cluster with TCP proxy
func (s *Specification) GetEtcdProxyClient(tlsCfg *tls.Config, tcpProxy *proxy.TCPProxy) (*clientv3.Client, chan struct{}, error) {
	closeC := tcpProxy.Run(s.GetPDList())
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: tcpProxy.GetEndpoints(),
		TLS:       tlsCfg,
	})
	return cli, closeC, err
}

// Merge returns a new Specification which sum old ones
func (s *Specification) Merge(that Topology) Topology {
	spec := that.(*Specification)
	return &Specification{
		GlobalOptions:    s.GlobalOptions,
		MonitoredOptions: s.MonitoredOptions,
		ServerConfigs:    s.ServerConfigs,
		TiDBServers:      append(s.TiDBServers, spec.TiDBServers...),
		TiKVServers:      append(s.TiKVServers, spec.TiKVServers...),
		PDServers:        append(s.PDServers, spec.PDServers...),
		TiFlashServers:   append(s.TiFlashServers, spec.TiFlashServers...),
		PumpServers:      append(s.PumpServers, spec.PumpServers...),
		Drainers:         append(s.Drainers, spec.Drainers...),
		CDCServers:       append(s.CDCServers, spec.CDCServers...),
		TiSparkMasters:   append(s.TiSparkMasters, spec.TiSparkMasters...),
		TiSparkWorkers:   append(s.TiSparkWorkers, spec.TiSparkWorkers...),
		Monitors:         append(s.Monitors, spec.Monitors...),
		Grafanas:         append(s.Grafanas, spec.Grafanas...),
		Alertmanagers:    append(s.Alertmanagers, spec.Alertmanagers...),
	}
}

// fillDefaults tries to fill custom fields to their default values
func fillCustomDefaults(globalOptions *GlobalOptions, data interface{}) error {
	v := reflect.ValueOf(data).Elem()
	t := v.Type()

	var err error
	for i := 0; i < t.NumField(); i++ {
		if err = setCustomDefaults(globalOptions, v.Field(i)); err != nil {
			return err
		}
	}

	return nil
}

var (
	globalOptionTypeName  = reflect.TypeOf(GlobalOptions{}).Name()
	monitorOptionTypeName = reflect.TypeOf(MonitoredOptions{}).Name()
	serverConfigsTypeName = reflect.TypeOf(ServerConfigs{}).Name()
)

// Skip global/monitored options
func isSkipField(field reflect.Value) bool {
	tp := field.Type().Name()
	return tp == globalOptionTypeName || tp == monitorOptionTypeName || tp == serverConfigsTypeName
}

func setDefaultDir(parent, role, port string, field reflect.Value) {
	if field.String() != "" {
		return
	}
	if defaults.CanUpdate(field.Interface()) {
		dir := fmt.Sprintf("%s-%s", role, port)
		field.Set(reflect.ValueOf(filepath.Join(parent, dir)))
	}
}

func setCustomDefaults(globalOptions *GlobalOptions, field reflect.Value) error {
	if !field.CanSet() || isSkipField(field) {
		return nil
	}

	switch field.Kind() {
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			if err := setCustomDefaults(globalOptions, field.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		ref := reflect.New(field.Type())
		ref.Elem().Set(field)
		if err := fillCustomDefaults(globalOptions, ref.Interface()); err != nil {
			return err
		}
		field.Set(ref.Elem())
	case reflect.Ptr:
		if err := setCustomDefaults(globalOptions, field.Elem()); err != nil {
			return err
		}
	}

	if field.Kind() != reflect.Struct {
		return nil
	}

	for j := 0; j < field.NumField(); j++ {
		switch field.Type().Field(j).Name {
		case "SSHPort":
			if field.Field(j).Int() != 0 {
				continue
			}
			field.Field(j).Set(reflect.ValueOf(globalOptions.SSHPort))
		case "Name":
			if field.Field(j).String() != "" {
				continue
			}
			host := reflect.Indirect(field).FieldByName("Host").String()
			clientPort := reflect.Indirect(field).FieldByName("ClientPort").Int()
			field.Field(j).Set(reflect.ValueOf(fmt.Sprintf("pd-%s-%d", host, clientPort)))
		case "DataDir":
			if reflect.Indirect(field).FieldByName("Imported").Interface().(bool) {
				setDefaultDir(globalOptions.DataDir, field.Addr().Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
			}

			dataDir := field.Field(j).String()

			// If the per-instance data_dir already have a value, skip filling default values
			// and ignore any value in global data_dir, the default values are filled only
			// when the pre-instance data_dir is empty
			if dataDir != "" {
				continue
			}
			// If the data dir in global options is an absolute path, append current
			// value to the global and has a comp-port sub directory
			if strings.HasPrefix(globalOptions.DataDir, "/") {
				field.Field(j).Set(reflect.ValueOf(filepath.Join(
					globalOptions.DataDir,
					fmt.Sprintf("%s-%s", field.Addr().Interface().(InstanceSpec).Role(), getPort(field)),
				)))
				continue
			}
			// If the data dir in global options is empty or a relative path, keep it be relative
			// Our run_*.sh start scripts are run inside deploy_path, so the final location
			// will be deploy_path/global.data_dir
			// (the default value of global.data_dir is "data")
			if globalOptions.DataDir == "" {
				field.Field(j).Set(reflect.ValueOf("data"))
			} else {
				field.Field(j).Set(reflect.ValueOf(globalOptions.DataDir))
			}
		case "DeployDir":
			setDefaultDir(globalOptions.DeployDir, field.Addr().Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
		case "LogDir":
			if reflect.Indirect(field).FieldByName("Imported").Interface().(bool) {
				setDefaultDir(globalOptions.LogDir, field.Addr().Interface().(InstanceSpec).Role(), getPort(field), field.Field(j))
			}

			logDir := field.Field(j).String()

			// If the per-instance log_dir already have a value, skip filling default values
			// and ignore any value in global log_dir, the default values are filled only
			// when the pre-instance log_dir is empty
			if logDir != "" {
				continue
			}
			// If the log dir in global options is an absolute path, append current
			// value to the global and has a comp-port sub directory
			if strings.HasPrefix(globalOptions.LogDir, "/") {
				field.Field(j).Set(reflect.ValueOf(filepath.Join(
					globalOptions.LogDir,
					fmt.Sprintf("%s-%s", field.Addr().Interface().(InstanceSpec).Role(), getPort(field)),
				)))
				continue
			}
			// If the log dir in global options is empty or a relative path, keep it be relative
			// Our run_*.sh start scripts are run inside deploy_path, so the final location
			// will be deploy_path/global.log_dir
			// (the default value of global.log_dir is "log")
			if globalOptions.LogDir == "" {
				field.Field(j).Set(reflect.ValueOf("log"))
			} else {
				field.Field(j).Set(reflect.ValueOf(globalOptions.LogDir))
			}
		case "Arch":
			switch strings.ToLower(field.Field(j).String()) {
			// replace "x86_64" with amd64, they are the same in our repo
			case "x86_64":
				field.Field(j).Set(reflect.ValueOf("amd64"))
			// replace "aarch64" with arm64
			case "aarch64":
				field.Field(j).Set(reflect.ValueOf("arm64"))
			}

			// convert to lower case
			if field.Field(j).String() != "" {
				field.Field(j).Set(reflect.ValueOf(strings.ToLower(field.Field(j).String())))
			}
		case "OS":
			// convert to lower case
			if field.Field(j).String() != "" {
				field.Field(j).Set(reflect.ValueOf(strings.ToLower(field.Field(j).String())))
			}
		}
	}

	return nil
}

func getPort(v reflect.Value) string {
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "Port", "ClientPort", "WebPort", "TCPPort", "NodeExporterPort":
			return fmt.Sprintf("%d", v.Field(i).Int())
		}
	}
	return ""
}

// ComponentsByStopOrder return component in the order need to stop.
func (s *Specification) ComponentsByStopOrder() (comps []Component) {
	comps = s.ComponentsByStartOrder()
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
func (s *Specification) ComponentsByStartOrder() (comps []Component) {
	// "pd", "tikv", "pump", "tidb", "tiflash", "drainer", "cdc", "prometheus", "grafana", "alertmanager"
	comps = append(comps, &PDComponent{s})
	comps = append(comps, &TiKVComponent{s})
	comps = append(comps, &PumpComponent{s})
	comps = append(comps, &TiDBComponent{s})
	comps = append(comps, &TiFlashComponent{s})
	comps = append(comps, &DrainerComponent{s})
	comps = append(comps, &CDCComponent{s})
	comps = append(comps, &MonitorComponent{s})
	comps = append(comps, &GrafanaComponent{s})
	comps = append(comps, &AlertManagerComponent{s})
	comps = append(comps, &TiSparkMasterComponent{s})
	comps = append(comps, &TiSparkWorkerComponent{s})
	return
}

// ComponentsByUpdateOrder return component in the order need to be updated.
func (s *Specification) ComponentsByUpdateOrder() (comps []Component) {
	// "tiflash", "pd", "tikv", "pump", "tidb", "drainer", "cdc", "prometheus", "grafana", "alertmanager"
	comps = append(comps, &TiFlashComponent{s})
	comps = append(comps, &PDComponent{s})
	comps = append(comps, &TiKVComponent{s})
	comps = append(comps, &PumpComponent{s})
	comps = append(comps, &TiDBComponent{s})
	comps = append(comps, &DrainerComponent{s})
	comps = append(comps, &CDCComponent{s})
	comps = append(comps, &MonitorComponent{s})
	comps = append(comps, &GrafanaComponent{s})
	comps = append(comps, &AlertManagerComponent{s})
	comps = append(comps, &TiSparkMasterComponent{s})
	comps = append(comps, &TiSparkWorkerComponent{s})
	return
}

// FindComponent returns the Component corresponding the name
func FindComponent(topo Topology, name string) Component {
	for _, com := range topo.ComponentsByStartOrder() {
		if com.Name() == name {
			return com
		}
	}
	return nil
}

// IterComponent iterates all components in component starting order
func (s *Specification) IterComponent(fn func(comp Component)) {
	for _, comp := range s.ComponentsByStartOrder() {
		fn(comp)
	}
}

// IterInstance iterates all instances in component starting order
func (s *Specification) IterInstance(fn func(instance Instance)) {
	for _, comp := range s.ComponentsByStartOrder() {
		for _, inst := range comp.Instances() {
			fn(inst)
		}
	}
}

// IterHost iterates one instance for each host
func IterHost(topo Topology, fn func(instance Instance)) {
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
func (s *Specification) Endpoints(user string) []*scripts.PDScript {
	var ends []*scripts.PDScript
	for _, spec := range s.PDServers {
		deployDir := Abs(user, spec.DeployDir)
		// data dir would be empty for components which don't need it
		dataDir := spec.DataDir
		// the default data_dir is relative to deploy_dir
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join(deployDir, dataDir)
		}
		// log dir will always be with values, but might not used by the component
		logDir := Abs(user, spec.LogDir)

		script := scripts.NewPDScript(
			spec.Name,
			spec.Host,
			deployDir,
			dataDir,
			logDir,
		).
			WithClientPort(spec.ClientPort).
			WithPeerPort(spec.PeerPort).
			WithListenHost(spec.ListenHost)
		if s.GlobalOptions.TLSEnabled {
			script = script.WithScheme("https")
		}
		script = script.WithAdvertiseClientAddr(spec.AdvertiseClientAddr).
			WithAdvertisePeerAddr(spec.AdvertisePeerAddr)
		ends = append(ends, script)
	}
	return ends
}

// AlertManagerEndpoints returns the AlertManager endpoints configurations
func AlertManagerEndpoints(alertmanager []*AlertmanagerSpec, user string, enableTLS bool) []*scripts.AlertManagerScript {
	var ends []*scripts.AlertManagerScript
	for _, spec := range alertmanager {
		deployDir := Abs(user, spec.DeployDir)
		// data dir would be empty for components which don't need it
		dataDir := spec.DataDir
		// the default data_dir is relative to deploy_dir
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join(deployDir, dataDir)
		}
		// log dir will always be with values, but might not used by the component
		logDir := Abs(user, spec.LogDir)

		script := scripts.NewAlertManagerScript(
			spec.Host,
			spec.ListenHost,
			deployDir,
			dataDir,
			logDir,
			enableTLS,
		).
			WithWebPort(spec.WebPort).
			WithClusterPort(spec.ClusterPort)
		ends = append(ends, script)
	}
	return ends
}

// FillHostArchOrOS fills the topology with the given host->arch
func (s *Specification) FillHostArchOrOS(hostArch map[string]string, fullType FullHostType) error {
	if err := FillHostArchOrOS(s, hostArch, fullType); err != nil {
		return err
	}

	return s.platformConflictsDetect()
}

// FillHostArchOrOS fills the topology with the given host->arch
func FillHostArchOrOS(s interface{}, hostArchOrOS map[string]string, fullType FullHostType) error {
	for host, arch := range hostArchOrOS {
		switch arch {
		case "x86_64":
			hostArchOrOS[host] = "amd64"
		case "aarch64":
			hostArchOrOS[host] = "arm64"
		default:
			hostArchOrOS[host] = strings.ToLower(arch)
		}
	}

	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Slice {
			continue
		}
		for j := 0; j < field.Len(); j++ {
			if err := setHostArchOrOS(field.Index(j), hostArchOrOS, fullType); err != nil {
				return err
			}
		}
	}
	return nil
}

func setHostArchOrOS(field reflect.Value, hostArchOrOS map[string]string, fullType FullHostType) error {
	if !field.CanSet() || isSkipField(field) {
		return nil
	}

	if field.Kind() == reflect.Ptr {
		return setHostArchOrOS(field.Elem(), hostArchOrOS, fullType)
	}

	if field.Kind() != reflect.Struct {
		return nil
	}

	host := field.FieldByName("Host")
	arch := field.FieldByName("Arch")
	os := field.FieldByName("OS")

	// set arch only if not set before
	if fullType == FullOSType {
		if !host.IsZero() && os.CanSet() && len(os.String()) == 0 {
			os.Set(reflect.ValueOf(hostArchOrOS[host.String()]))
		}
	} else {
		if !host.IsZero() && arch.CanSet() && len(arch.String()) == 0 {
			arch.Set(reflect.ValueOf(hostArchOrOS[host.String()]))
		}
	}

	return nil
}

// when upgrade form old tiup-cluster, replace spec.CommitTS with spec.Config["initial-commit-ts"]
func (s *Specification) removeCommitTS() {
	_, ok1 := s.ServerConfigs.Drainer["initial-commit-ts"]
	for _, spec := range s.Drainers {
		_, ok2 := spec.Config["initial-commit-ts"]
		if !ok1 && !ok2 && spec.CommitTS != nil && *spec.CommitTS != -1 {
			if spec.Config == nil {
				spec.Config = make(map[string]interface{})
			}
			spec.Config["initial-commit-ts"] = *spec.CommitTS
		}
		spec.CommitTS = nil
	}
}

// GetGrafanaConfig returns global grafana configurations
func (s *Specification) GetGrafanaConfig() map[string]string {
	return s.ServerConfigs.Grafana
}
