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

package scripts

import (
	"bytes"
	"os"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
)

// GrafanaScript represent the data to generate Grafana config
type GrafanaScript struct {
	ClusterName string
	DeployDir   string
	NumaNode    string
	tplName     string
}

// NewGrafanaScript returns a GrafanaScript with given arguments
func NewGrafanaScript(cluster, deployDir string) *GrafanaScript {
	return &GrafanaScript{
		ClusterName: cluster,
		DeployDir:   deployDir,
	}
}

// WithNumaNode set NumaNode field of GrafanaScript
func (c *GrafanaScript) WithNumaNode(numa string) *GrafanaScript {
	c.NumaNode = numa
	return c
}

// WithTPLFile set the template file.
func (c *GrafanaScript) WithTPLFile(file string) *GrafanaScript {
	c.tplName = file
	return c
}

// Config generate the config file data.
func (c *GrafanaScript) Config() ([]byte, error) {
	fp := c.tplName
	if fp == "" {
		fp = path.Join("templates", "scripts", "run_grafana.sh.tpl")
	}

	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Grafana config content by tpl
func (c *GrafanaScript) ConfigWithTemplate(tpl string) ([]byte, error) {
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
func (c *GrafanaScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}
