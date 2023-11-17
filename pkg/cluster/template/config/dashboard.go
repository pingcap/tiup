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

// DashboardConfig represent the data to generate Dashboard config
type DashboardConfig struct {
	ClusterName string
	DeployDir   string
}

// NewDashboardConfig returns a DashboardConfig
func NewDashboardConfig(cluster, deployDir string) *DashboardConfig {
	return &DashboardConfig{
		ClusterName: cluster,
		DeployDir:   deployDir,
	}
}

// Config generate the config file data.
func (c *DashboardConfig) Config() ([]byte, error) {
	fp := path.Join("templates", "config", "dashboard.yml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Dashboard config content by tpl
func (c *DashboardConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("dashboard").Parse(tpl)
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
func (c *DashboardConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, config, 0755)
}
