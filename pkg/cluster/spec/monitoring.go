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
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"gopkg.in/yaml.v3"
)

// PrometheusSpec represents the Prometheus Server topology specification in topology.yaml
type PrometheusSpec struct {
	Host                  string                 `yaml:"host"`
	SSHPort               int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported              bool                   `yaml:"imported,omitempty"`
	Patched               bool                   `yaml:"patched,omitempty"`
	IgnoreExporter        bool                   `yaml:"ignore_exporter,omitempty"`
	Port                  int                    `yaml:"port" default:"9090"`
	NgPort                int                    `yaml:"ng_port,omitempty" validate:"ng_port:editable"` // ng_port is usable since v5.3.0 and default as 12020 since v5.4.0, so the default value is set in spec.go/AdjustByVersion
	DeployDir             string                 `yaml:"deploy_dir,omitempty"`
	DataDir               string                 `yaml:"data_dir,omitempty"`
	LogDir                string                 `yaml:"log_dir,omitempty"`
	NumaNode              string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	RemoteConfig          Remote                 `yaml:"remote_config,omitempty" validate:"remote_config:ignore"`
	ExternalAlertmanagers []ExternalAlertmanager `yaml:"external_alertmanagers" validate:"external_alertmanagers:ignore"`
	Retention             string                 `yaml:"storage_retention,omitempty" validate:"storage_retention:editable"`
	ResourceControl       meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch                  string                 `yaml:"arch,omitempty"`
	OS                    string                 `yaml:"os,omitempty"`
	RuleDir               string                 `yaml:"rule_dir,omitempty" validate:"rule_dir:editable"`
	AdditionalScrapeConf  map[string]interface{} `yaml:"additional_scrape_conf,omitempty" validate:"additional_scrape_conf:ignore"`
}

// Remote prometheus remote config
type Remote struct {
	RemoteWrite []map[string]interface{} `yaml:"remote_write,omitempty" validate:"remote_write:ignore"`
	RemoteRead  []map[string]interface{} `yaml:"remote_read,omitempty" validate:"remote_read:ignore"`
}

// ExternalAlertmanager configs prometheus to include alertmanagers not deployed in current cluster
type ExternalAlertmanager struct {
	Host    string `yaml:"host"`
	WebPort int    `yaml:"web_port" default:"9093"`
}

// Role returns the component role of the instance
func (s *PrometheusSpec) Role() string {
	return ComponentPrometheus
}

// SSH returns the host and SSH port of the instance
func (s *PrometheusSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *PrometheusSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *PrometheusSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *PrometheusSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// MonitorComponent represents Monitor component.
type MonitorComponent struct{ Topology }

// Name implements Component interface.
func (c *MonitorComponent) Name() string {
	return ComponentPrometheus
}

// Role implements Component interface.
func (c *MonitorComponent) Role() string {
	return RoleMonitor
}

// Instances implements Component interface.
func (c *MonitorComponent) Instances() []Instance {
	servers := c.BaseTopo().Monitors
	ins := make([]Instance, 0, len(servers))

	for _, s := range servers {
		s := s
		mi := &MonitorInstance{BaseInstance{
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
			StatusFn: func(_ context.Context, _ *tls.Config, _ ...string) string {
				return statusByHost(s.Host, s.Port, "/-/ready", nil)
			},
			UptimeFn: func(_ context.Context, tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.Host, s.Port, tlsCfg)
			},
		}, c.Topology}
		if s.NgPort > 0 {
			mi.BaseInstance.Ports = append(mi.BaseInstance.Ports, s.NgPort)
		}
		ins = append(ins, mi)
	}
	return ins
}

// MonitorInstance represent the monitor instance
type MonitorInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *MonitorInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	gOpts := *i.topo.BaseTopo().GlobalOptions
	if err := i.BaseInstance.InitConfig(ctx, e, gOpts, deployUser, paths); err != nil {
		return err
	}

	enableTLS := gOpts.TLSEnabled
	// transfer run script
	spec := i.InstanceSpec.(*PrometheusSpec)
	cfg := scripts.NewPrometheusScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).
		WithNumaNode(spec.NumaNode).
		WithRetention(spec.Retention).
		WithNG(spec.NgPort)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_prometheus_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_prometheus.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	topoHasField := func(field string) (reflect.Value, bool) {
		return findSliceField(i.topo, field)
	}
	monitoredOptions := i.topo.GetMonitoredOptions()

	// transfer config
	cfig := config.NewPrometheusConfig(clusterName, clusterVersion, enableTLS)
	if monitoredOptions != nil {
		cfig.AddBlackbox(i.GetHost(), uint64(monitoredOptions.BlackboxExporterPort))
	}
	uniqueHosts := set.NewStringSet()

	if servers, found := topoHasField("PDServers"); found {
		for i := 0; i < servers.Len(); i++ {
			pd := servers.Index(i).Interface().(*PDSpec)
			uniqueHosts.Insert(pd.Host)
			cfig.AddPD(pd.Host, uint64(pd.ClientPort))
		}
	}
	if servers, found := topoHasField("TiKVServers"); found {
		for i := 0; i < servers.Len(); i++ {
			kv := servers.Index(i).Interface().(*TiKVSpec)
			uniqueHosts.Insert(kv.Host)
			cfig.AddTiKV(kv.Host, uint64(kv.StatusPort))
		}
	}
	if servers, found := topoHasField("TiDBServers"); found {
		for i := 0; i < servers.Len(); i++ {
			db := servers.Index(i).Interface().(*TiDBSpec)
			uniqueHosts.Insert(db.Host)
			cfig.AddTiDB(db.Host, uint64(db.StatusPort))
		}
	}
	if servers, found := topoHasField("TiFlashServers"); found {
		for i := 0; i < servers.Len(); i++ {
			flash := servers.Index(i).Interface().(*TiFlashSpec)
			uniqueHosts.Insert(flash.Host)
			cfig.AddTiFlashLearner(flash.Host, uint64(flash.FlashProxyStatusPort))
			cfig.AddTiFlash(flash.Host, uint64(flash.StatusPort))
		}
	}
	if servers, found := topoHasField("PumpServers"); found {
		for i := 0; i < servers.Len(); i++ {
			pump := servers.Index(i).Interface().(*PumpSpec)
			uniqueHosts.Insert(pump.Host)
			cfig.AddPump(pump.Host, uint64(pump.Port))
		}
	}
	if servers, found := topoHasField("Drainers"); found {
		for i := 0; i < servers.Len(); i++ {
			drainer := servers.Index(i).Interface().(*DrainerSpec)
			uniqueHosts.Insert(drainer.Host)
			cfig.AddDrainer(drainer.Host, uint64(drainer.Port))
		}
	}
	if servers, found := topoHasField("CDCServers"); found {
		for i := 0; i < servers.Len(); i++ {
			cdc := servers.Index(i).Interface().(*CDCSpec)
			uniqueHosts.Insert(cdc.Host)
			cfig.AddCDC(cdc.Host, uint64(cdc.Port))
		}
	}
	if servers, found := topoHasField("Monitors"); found {
		for i := 0; i < servers.Len(); i++ {
			monitoring := servers.Index(i).Interface().(*PrometheusSpec)
			uniqueHosts.Insert(monitoring.Host)
		}
	}
	if servers, found := topoHasField("Grafanas"); found {
		for i := 0; i < servers.Len(); i++ {
			grafana := servers.Index(i).Interface().(*GrafanaSpec)
			uniqueHosts.Insert(grafana.Host)
			cfig.AddGrafana(grafana.Host, uint64(grafana.Port))
		}
	}
	if servers, found := topoHasField("Alertmanagers"); found {
		for i := 0; i < servers.Len(); i++ {
			alertmanager := servers.Index(i).Interface().(*AlertmanagerSpec)
			uniqueHosts.Insert(alertmanager.Host)
			cfig.AddAlertmanager(alertmanager.Host, uint64(alertmanager.WebPort))
		}
	}
	if servers, found := topoHasField("Masters"); found {
		for i := 0; i < servers.Len(); i++ {
			master := reflect.Indirect(servers.Index(i))
			host, port := master.FieldByName("Host").String(), master.FieldByName("Port").Int()
			uniqueHosts.Insert(host)
			cfig.AddDMMaster(host, uint64(port))
		}
	}

	if servers, found := topoHasField("Workers"); found {
		for i := 0; i < servers.Len(); i++ {
			worker := reflect.Indirect(servers.Index(i))
			host, port := worker.FieldByName("Host").String(), worker.FieldByName("Port").Int()
			uniqueHosts.Insert(host)
			cfig.AddDMWorker(host, uint64(port))
		}
	}

	if monitoredOptions != nil {
		for host := range uniqueHosts {
			cfig.AddNodeExpoertor(host, uint64(monitoredOptions.NodeExporterPort))
			cfig.AddBlackboxExporter(host, uint64(monitoredOptions.BlackboxExporterPort))
			cfig.AddMonitoredServer(host)
		}
	}

	remoteCfg, err := encodeRemoteCfg2Yaml(spec.RemoteConfig)
	if err != nil {
		return err
	}
	cfig.SetRemoteConfig(string(remoteCfg))

	// doesn't work
	if _, err := i.setTLSConfig(ctx, false, nil, paths); err != nil {
		return err
	}

	for _, alertmanager := range spec.ExternalAlertmanagers {
		cfig.AddAlertmanager(alertmanager.Host, uint64(alertmanager.WebPort))
	}

	if spec.RuleDir != "" {
		filter := func(name string) bool { return strings.HasSuffix(name, ".rules.yml") }
		err := i.IteratorLocalConfigDir(ctx, spec.RuleDir, filter, func(name string) error {
			cfig.AddLocalRule(name)
			return nil
		})
		if err != nil {
			return errors.Annotate(err, "add local rule")
		}
	}

	if err := i.installRules(ctx, e, paths.Deploy, clusterName, clusterVersion); err != nil {
		return errors.Annotate(err, "install rules")
	}

	if err := i.initRules(ctx, e, spec, paths, clusterName); err != nil {
		return err
	}

	if spec.NgPort > 0 {
		ngcfg := config.NewNgMonitoringConfig(clusterName, clusterVersion, enableTLS)
		if servers, found := topoHasField("PDServers"); found {
			for i := 0; i < servers.Len(); i++ {
				pd := servers.Index(i).Interface().(*PDSpec)
				ngcfg.AddPD(pd.Host, uint64(pd.ClientPort))
			}
		}
		ngcfg.AddIP(i.GetHost()).
			AddPort(spec.NgPort).
			AddDeployDir(paths.Deploy).
			AddDataDir(paths.Data[0]).
			AddLog(paths.Log)

		if servers, found := topoHasField("Monitors"); found {
			for i := 0; i < servers.Len(); i++ {
				monitoring := servers.Index(i).Interface().(*PrometheusSpec)
				cfig.AddNGMonitoring(monitoring.Host, uint64(monitoring.NgPort))
			}
		}
		fp = filepath.Join(paths.Cache, fmt.Sprintf("ngmonitoring_%s_%d.toml", i.GetHost(), i.GetPort()))
		if err := ngcfg.ConfigToFile(fp); err != nil {
			return err
		}
		dst = filepath.Join(paths.Deploy, "conf", "ngmonitoring.toml")
		if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
			return err
		}
	}

	fp = filepath.Join(paths.Cache, fmt.Sprintf("prometheus_%s_%d.yml", i.GetHost(), i.GetPort()))
	if err := cfig.ConfigToFile(fp); err != nil {
		return err
	}
	if spec.AdditionalScrapeConf != nil {
		err = mergeAdditionalScrapeConf(fp, spec.AdditionalScrapeConf)
		if err != nil {
			return err
		}
	}
	dst = filepath.Join(paths.Deploy, "conf", "prometheus.yml")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".yml", paths, nil)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *MonitorInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]interface{}, paths meta.DirPaths) (map[string]interface{}, error) {
	return nil, nil
}

// We only really installRules for dm cluster because the rules(*.rules.yml) packed with the prometheus
// component is designed for tidb cluster (the dm cluster use the same prometheus component with tidb
// cluster), and the rules for dm cluster is packed in the dm-master component. So if deploying tidb
// cluster, the rules is correct, if deploying dm cluster, we should remove rules for tidb and install
// rules for dm.
func (i *MonitorInstance) installRules(ctx context.Context, e ctxt.Executor, deployDir, clusterName, clusterVersion string) error {
	if i.topo.Type() != TopoTypeDM {
		return nil
	}

	tmp := filepath.Join(deployDir, "_tiup_tmp")
	_, stderr, err := e.Execute(ctx, fmt.Sprintf("mkdir -p %s", tmp), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	srcPath := PackagePath(ComponentDMMaster, clusterVersion, i.OS(), i.Arch())
	dstPath := filepath.Join(tmp, filepath.Base(srcPath))

	err = e.Transfer(ctx, srcPath, dstPath, false, 0, false)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(`tar --no-same-owner -zxf %s -C %s && rm %s`, dstPath, tmp, dstPath)
	_, stderr, err = e.Execute(ctx, cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// copy dm-master/conf/*.rules.yml
	targetDir := filepath.Join(deployDir, "bin", "prometheus")
	cmds := []string{
		"mkdir -p %[1]s",
		`find %[1]s -type f -name "*.rules.yml" -delete`,
		`find %[2]s/dm-master/conf -type f -name "*.rules.yml" -exec cp {} %[1]s \;`,
		"rm -rf %[2]s",
		`find %[1]s -maxdepth 1 -type f -name "*.rules.yml" -exec sed -i 's/ENV_LABELS_ENV/%[3]s/g' {} \;`,
	}
	_, stderr, err = e.Execute(ctx, fmt.Sprintf(strings.Join(cmds, " && "), targetDir, tmp, clusterName), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	return nil
}

func (i *MonitorInstance) initRules(ctx context.Context, e ctxt.Executor, spec *PrometheusSpec, paths meta.DirPaths, clusterName string) error {
	// To make this step idempotent, we need cleanup old rules first
	cmds := []string{
		"mkdir -p %[1]s/conf",
		`find %[1]s/conf -type f -name "*.rules.yml" -delete`,
		`find %[1]s/bin/prometheus -maxdepth 1 -type f -name "*.rules.yml" -exec cp {} %[1]s/conf/ \;`,
		`find %[1]s/conf -maxdepth 1 -type f -name "*.rules.yml" -exec sed -i -e 's/ENV_LABELS_ENV/%[2]s/g' {} \;`,
	}

	_, stderr, err := e.Execute(ctx, fmt.Sprintf(strings.Join(cmds, " && "), paths.Deploy, clusterName), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// render cluster name when monitoring_servers.rule_dir is set
	if spec.RuleDir != "" {
		err := i.TransferLocalConfigDir(ctx, e, spec.RuleDir, path.Join(paths.Deploy, "conf"), func(name string) bool {
			return strings.HasSuffix(name, ".rules.yml")
		})
		if err != nil {
			return err
		}
		// only need to render the cluster name
		cmds = []string{
			`find %[1]s/conf -maxdepth 1 -type f -name "*.rules.yml" -exec sed -i -e 's/env: [^ ]*/env: %[2]s/g' {} \;`,
			`find %[1]s/conf -maxdepth 1 -type f -name "*.rules.yml" -exec sed -i -e 's/cluster: [^ ]*,/cluster: %[2]s,/g' {} \;`,
		}
		_, stderr, err := e.Execute(ctx, fmt.Sprintf(strings.Join(cmds, " && "), paths.Deploy, clusterName), false)
		if err != nil {
			return errors.Annotatef(err, "stderr: %s", string(stderr))
		}
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *MonitorInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName string,
	clusterVersion string,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

func mergeAdditionalScrapeConf(source string, addition map[string]interface{}) error {
	var result map[string]interface{}
	bytes, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(bytes, &result)
	if err != nil {
		return err
	}

	for _, job := range result["scrape_configs"].([]interface{}) {
		for k, v := range addition {
			job.(map[string]interface{})[k] = v
		}
	}
	bytes, err = yaml.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(source, bytes, 0644)
}
