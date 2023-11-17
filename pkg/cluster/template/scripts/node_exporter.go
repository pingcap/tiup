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
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// NodeExporterScript represent the data to generate NodeExporter config
type NodeExporterScript struct {
	Port      uint64
	DeployDir string
	LogDir    string
	NumaNode  string
}

// NewNodeExporterScript returns a NodeExporterScript with given arguments
func NewNodeExporterScript(deployDir, logDir string) *NodeExporterScript {
	return &NodeExporterScript{
		Port:      9100,
		DeployDir: deployDir,
		LogDir:    logDir,
	}
}

// WithPort set Port field of NodeExporterScript
func (c *NodeExporterScript) WithPort(port uint64) *NodeExporterScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of NodeExporterScript
func (c *NodeExporterScript) WithNumaNode(numa string) *NodeExporterScript {
	c.NumaNode = numa
	return c
}

// Config generate the config file data.
func (c *NodeExporterScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_node_exporter.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *NodeExporterScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the NodeExporter config content by tpl
func (c *NodeExporterScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("NodeExporter").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
