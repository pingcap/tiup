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
	"os"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
)

// DatasourceConfig represent the data to generate Datasource config
type DatasourceConfig struct {
	ClusterName string
	IP          string
	Port        uint64
}

// NewDatasourceConfig returns a DatasourceConfig
func NewDatasourceConfig(cluster, ip string) *DatasourceConfig {
	return &DatasourceConfig{
		ClusterName: cluster,
		IP:          ip,
		Port:        9090,
	}
}

// WithPort set Port field of DatasourceConfig
func (c *DatasourceConfig) WithPort(port uint64) *DatasourceConfig {
	c.Port = port
	return c
}

// Config generate the config file data.
func (c *DatasourceConfig) Config() ([]byte, error) {
	fp := path.Join("templates", "config", "datasource.yml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Datasource config content by tpl
func (c *DatasourceConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Datasource").Parse(tpl)
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
func (c *DatasourceConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}
