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
	"io/ioutil"
	"path"
	"text/template"

	"github.com/pingcap/tiup/pkg/cluster/embed"
)

// GrafanaConfig represent the data to generate Grafana config
type GrafanaConfig struct {
	DeployDir string
	IP        string
	Port      uint64
	Username  string // admin_user
	Password  string // admin_password
}

// NewGrafanaConfig returns a GrafanaConfig
func NewGrafanaConfig(ip, deployDir string) *GrafanaConfig {
	return &GrafanaConfig{
		DeployDir: deployDir,
		IP:        ip,
		Port:      3000,
	}
}

// WithPort set Port field of GrafanaConfig
func (c *GrafanaConfig) WithPort(port uint64) *GrafanaConfig {
	c.Port = port
	return c
}

// WithUsername sets username of admin user
func (c *GrafanaConfig) WithUsername(user string) *GrafanaConfig {
	c.Username = user
	return c
}

// WithPassword sets password of admin user
func (c *GrafanaConfig) WithPassword(passwd string) *GrafanaConfig {
	c.Password = passwd
	return c
}

// Config generate the config file data.
func (c *GrafanaConfig) Config() ([]byte, error) {
	fp := path.Join("/templates", "config", "grafana.ini.tpl")
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Grafana config content by tpl
func (c *GrafanaConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Grafana").Parse(tpl)
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
func (c *GrafanaConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}
