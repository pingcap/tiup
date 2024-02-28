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

package config

import (
	"bytes"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

// PrometheusConfig represent the data to generate Prometheus config
type PrometheusConfig struct {
	ClusterName               string
	ScrapeInterval            string
	ScrapeTimeout             string
	TLSEnabled                bool
	NodeExporterAddrs         []string
	TiDBStatusAddrs           []string
	TiProxyStatusAddrs        []string
	TiKVStatusAddrs           []string
	PDAddrs                   []string
	TSOAddrs                  []string
	SchedulingAddrs           []string
	TiFlashStatusAddrs        []string
	TiFlashLearnerStatusAddrs []string
	PumpAddrs                 []string
	DrainerAddrs              []string
	CDCAddrs                  []string
	TiKVCDCAddrs              []string
	BlackboxExporterAddrs     []string
	LightningAddrs            []string
	MonitoredServers          []string
	AlertmanagerAddrs         []string
	NGMonitoringAddrs         []string
	PushgatewayAddrs          []string
	BlackboxAddr              string
	GrafanaAddr               string
	HasTiKVAccelerateRules    bool

	DMMasterAddrs []string
	DMWorkerAddrs []string

	LocalRules   []string
	RemoteConfig string
}

// NewPrometheusConfig returns a PrometheusConfig
func NewPrometheusConfig(clusterName, clusterVersion string, enableTLS bool) *PrometheusConfig {
	cfg := &PrometheusConfig{
		ClusterName:            clusterName,
		TLSEnabled:             enableTLS,
		HasTiKVAccelerateRules: tidbver.PrometheusHasTiKVAccelerateRules(clusterVersion),
	}

	return cfg
}

// AddNodeExpoertor add a node expoter address
func (c *PrometheusConfig) AddNodeExpoertor(ip string, port uint64) *PrometheusConfig {
	c.NodeExporterAddrs = append(c.NodeExporterAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddTiDB add a TiDB address
func (c *PrometheusConfig) AddTiDB(ip string, port uint64) *PrometheusConfig {
	c.TiDBStatusAddrs = append(c.TiDBStatusAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddTiProxy add a TiProxy address
func (c *PrometheusConfig) AddTiProxy(ip string, port uint64) *PrometheusConfig {
	c.TiProxyStatusAddrs = append(c.TiProxyStatusAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddTiKV add a TiKV address
func (c *PrometheusConfig) AddTiKV(ip string, port uint64) *PrometheusConfig {
	c.TiKVStatusAddrs = append(c.TiKVStatusAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddPD add a PD address
func (c *PrometheusConfig) AddPD(ip string, port uint64) *PrometheusConfig {
	c.PDAddrs = append(c.PDAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddTSO add a TSO address
func (c *PrometheusConfig) AddTSO(ip string, port uint64) *PrometheusConfig {
	c.TSOAddrs = append(c.TSOAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddScheduling add a scheduling address
func (c *PrometheusConfig) AddScheduling(ip string, port uint64) *PrometheusConfig {
	c.SchedulingAddrs = append(c.SchedulingAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddTiFlashLearner add a TiFlash learner address
func (c *PrometheusConfig) AddTiFlashLearner(ip string, port uint64) *PrometheusConfig {
	c.TiFlashLearnerStatusAddrs = append(c.TiFlashLearnerStatusAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddTiFlash add a TiFlash address
func (c *PrometheusConfig) AddTiFlash(ip string, port uint64) *PrometheusConfig {
	c.TiFlashStatusAddrs = append(c.TiFlashStatusAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddPump add a pump address
func (c *PrometheusConfig) AddPump(ip string, port uint64) *PrometheusConfig {
	c.PumpAddrs = append(c.PumpAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddDrainer add a drainer address
func (c *PrometheusConfig) AddDrainer(ip string, port uint64) *PrometheusConfig {
	c.DrainerAddrs = append(c.DrainerAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddCDC add a cdc address
func (c *PrometheusConfig) AddCDC(ip string, port uint64) *PrometheusConfig {
	c.CDCAddrs = append(c.CDCAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddTiKVCDC add a tikv-cdc address
func (c *PrometheusConfig) AddTiKVCDC(ip string, port uint64) *PrometheusConfig {
	c.TiKVCDCAddrs = append(c.TiKVCDCAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddBlackboxExporter add a BlackboxExporter address
func (c *PrometheusConfig) AddBlackboxExporter(ip string, port uint64) *PrometheusConfig {
	c.BlackboxExporterAddrs = append(c.BlackboxExporterAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddLightning add a lightning address
func (c *PrometheusConfig) AddLightning(ip string, port uint64) *PrometheusConfig {
	c.LightningAddrs = append(c.LightningAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddMonitoredServer add a MonitoredServer address
func (c *PrometheusConfig) AddMonitoredServer(ip string) *PrometheusConfig {
	c.MonitoredServers = append(c.MonitoredServers, ip)
	return c
}

// AddAlertmanager add an alertmanager address
func (c *PrometheusConfig) AddAlertmanager(ip string, port uint64) *PrometheusConfig {
	c.AlertmanagerAddrs = append(c.AlertmanagerAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddPushgateway add an pushgateway address
func (c *PrometheusConfig) AddPushgateway(addresses []string) *PrometheusConfig {
	c.PushgatewayAddrs = addresses
	return c
}

// AddBlackbox add an blackbox address
func (c *PrometheusConfig) AddBlackbox(ip string, port uint64) *PrometheusConfig {
	c.BlackboxAddr = utils.JoinHostPort(ip, int(port))
	return c
}

// AddNGMonitoring add an ng-monitoring server exporter address
func (c *PrometheusConfig) AddNGMonitoring(ip string, port uint64) *PrometheusConfig {
	c.NGMonitoringAddrs = append(c.NGMonitoringAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddGrafana add an Grafana address
func (c *PrometheusConfig) AddGrafana(ip string, port uint64) *PrometheusConfig {
	c.GrafanaAddr = utils.JoinHostPort(ip, int(port))
	return c
}

// AddDMMaster add an dm-master address
func (c *PrometheusConfig) AddDMMaster(ip string, port uint64) *PrometheusConfig {
	c.DMMasterAddrs = append(c.DMMasterAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddDMWorker add an dm-worker address
func (c *PrometheusConfig) AddDMWorker(ip string, port uint64) *PrometheusConfig {
	c.DMWorkerAddrs = append(c.DMWorkerAddrs, utils.JoinHostPort(ip, int(port)))
	return c
}

// AddLocalRule add a local rule
func (c *PrometheusConfig) AddLocalRule(rule string) *PrometheusConfig {
	c.LocalRules = append(c.LocalRules, rule)
	return c
}

// SetRemoteConfig set remote read/write config
func (c *PrometheusConfig) SetRemoteConfig(cfg string) *PrometheusConfig {
	c.RemoteConfig = cfg
	return c
}

// Config generate the config file data.
func (c *PrometheusConfig) Config() ([]byte, error) {
	fp := path.Join("templates", "config", "prometheus.yml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Prometheus config content by tpl
func (c *PrometheusConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Prometheus").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// ConfigToFile write config content to specific path
func (c *PrometheusConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, config, 0755)
}
