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

package dm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"text/template"

	"github.com/pingcap/tiup/pkg/cluster/embed"
)

// PrometheusConfig represents the data to generate Prometheus config
// You may need to update the template file if change this struct.
type PrometheusConfig struct {
	ClusterName       string
	TLSEnabled        bool
	AlertmanagerAddrs []string
	GrafanaAddr       string

	MasterAddrs []string
	WorkerAddrs []string
}

// NewPrometheusConfig returns a PrometheusConfig
func NewPrometheusConfig(cluster string, enableTLS bool) *PrometheusConfig {
	return &PrometheusConfig{
		ClusterName: cluster,
		TLSEnabled:  enableTLS,
	}
}

// AddGrafana adds an kafka exporter address
func (c *PrometheusConfig) AddGrafana(ip string, port uint64) *PrometheusConfig {
	c.GrafanaAddr = fmt.Sprintf("%s:%d", ip, port)
	return c
}

// AddAlertmanager add an alertmanager address
func (c *PrometheusConfig) AddAlertmanager(ip string, port uint64) *PrometheusConfig {
	c.AlertmanagerAddrs = append(c.AlertmanagerAddrs, fmt.Sprintf("%s:%d", ip, port))
	return c
}

// AddMasterAddrs add an dm-master address
func (c *PrometheusConfig) AddMasterAddrs(ip string, port uint64) *PrometheusConfig {
	c.MasterAddrs = append(c.MasterAddrs, fmt.Sprintf("%s:%d", ip, port))
	return c
}

// AddWorkerAddrs add an dm-worker address
func (c *PrometheusConfig) AddWorkerAddrs(ip string, port uint64) *PrometheusConfig {
	c.WorkerAddrs = append(c.WorkerAddrs, fmt.Sprintf("%s:%d", ip, port))
	return c
}

// Config generate the config file data.
func (c *PrometheusConfig) Config() ([]byte, error) {
	fp := path.Join("/templates", "config", "dm", "prometheus.yml.tpl")
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Prometheus config content by tpl
func (c *PrometheusConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Prometheus-dm").Parse(tpl)
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
	return ioutil.WriteFile(file, config, 0755)
}
