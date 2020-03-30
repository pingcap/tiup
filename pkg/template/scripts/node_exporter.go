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
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
)

// NodeExporterScript represent the data to generate NodeExporter config
type NodeExporterScript struct {
	DeployDir string
	NumaNode  string
	Port      uint64
}

// NewNodeExporterScript returns a NodeExporterScript with given arguments
func NewNodeExporterScript(deployDir string) *NodeExporterScript {
	return &NodeExporterScript{
		DeployDir: deployDir,
		Port:      9100,
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

// Config read ${localdata.EnvNameComponentInstallDir}/templates/scripts/run_node_exporter.sh.tpl as template
// and generate the config by ConfigWithTemplate
func (c *NodeExporterScript) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "scripts", "run_node_exporter.sh.tpl")
	tpl, err := ioutil.ReadFile(fp)
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
	return ioutil.WriteFile(file, config, 0755)
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
