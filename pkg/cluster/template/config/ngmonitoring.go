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
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// NgMonitoringConfig represent the data to generate NgMonitoring config
type NgMonitoringConfig struct {
	ClusterName string
	TLSEnabled  bool
	IP          string
	Port        int
	PDAddrs     string
	DeployDir   string
	DataDir     string
	LogDir      string
}

// NewNgMonitoringConfig returns a PrometheusConfig
func NewNgMonitoringConfig(clusterName, clusterVersion string, enableTLS bool) *NgMonitoringConfig {
	cfg := &NgMonitoringConfig{
		ClusterName: clusterName,
		TLSEnabled:  enableTLS,
	}
	return cfg
}

// AddPD add a PD address
func (c *NgMonitoringConfig) AddPD(ip string, port uint64) *NgMonitoringConfig {
	if c.PDAddrs == "" {
		c.PDAddrs = fmt.Sprintf("\"%s\"", utils.JoinHostPort(ip, int(port)))
	} else {
		c.PDAddrs += fmt.Sprintf(",\"%s\"", utils.JoinHostPort(ip, int(port)))
	}
	return c
}

// AddIP add ip to ng-monitoring conf
func (c *NgMonitoringConfig) AddIP(ip string) *NgMonitoringConfig {
	c.IP = ip
	return c
}

// AddLog add logdir to ng-monitoring conf
func (c *NgMonitoringConfig) AddLog(dir string) *NgMonitoringConfig {
	c.LogDir = dir
	return c
}

// AddDeployDir add logdir to ng-monitoring conf
func (c *NgMonitoringConfig) AddDeployDir(dir string) *NgMonitoringConfig {
	c.DeployDir = dir
	return c
}

// AddDataDir add logdir to ng-monitoring conf
func (c *NgMonitoringConfig) AddDataDir(dir string) *NgMonitoringConfig {
	c.DataDir = dir
	return c
}

// AddPort add port to ng-monitoring conf
func (c *NgMonitoringConfig) AddPort(port int) *NgMonitoringConfig {
	c.Port = port
	return c
}

// ConfigWithTemplate generate the Prometheus config content by tpl
func (c *NgMonitoringConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("NgMonitoring").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// Config generate the config file data.
func (c *NgMonitoringConfig) Config() ([]byte, error) {
	fp := path.Join("templates", "config", "ngmonitoring.toml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *NgMonitoringConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}
