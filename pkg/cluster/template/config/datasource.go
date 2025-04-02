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
	"github.com/pingcap/tiup/pkg/utils"
)

// DatasourceConfig represent the data to generate Datasource config
type DatasourceConfig struct {
	ClusterName string
	URL         string
	Name        string
	Type        string
	IsDefault   bool
}

// NewDatasourceConfig returns a DatasourceConfig
func NewDatasourceConfig(clusterName, url string) *DatasourceConfig {
	return &DatasourceConfig{
		ClusterName: clusterName,
		URL:         url,
		Name:        clusterName,
		Type:        "prometheus",
		IsDefault:   true,
	}
}

// WithName sets name of datasource
func (c *DatasourceConfig) WithName(name string) *DatasourceConfig {
	c.Name = name
	return c
}

// WithType sets type of datasource
func (c *DatasourceConfig) WithType(typ string) *DatasourceConfig {
	c.Type = typ
	return c
}

// WithIsDefault sets if datasource is default
func (c *DatasourceConfig) WithIsDefault(isDefault bool) *DatasourceConfig {
	c.IsDefault = isDefault
	return c
}

// ConfigToFile write config content to specific path
func (c *DatasourceConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, config, 0755)
}

// Config generate the config file data.
func (c *DatasourceConfig) Config() ([]byte, error) {
	fp := path.Join("templates", "config", "datasource.yml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("Datasource").Parse(string(tpl))
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
