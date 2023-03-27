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

// BlackboxExporterScript represent the data to generate BlackboxExporter config
type BlackboxExporterScript struct {
	Port      uint64
	DeployDir string
	LogDir    string
	NumaNode  string
}

// NewBlackboxExporterScript returns a BlackboxExporterScript with given arguments
func NewBlackboxExporterScript(deployDir, logDir string) *BlackboxExporterScript {
	return &BlackboxExporterScript{
		Port:      9115,
		DeployDir: deployDir,
		LogDir:    logDir,
	}
}

// WithPort set WebPort field of BlackboxExporterScript
func (c *BlackboxExporterScript) WithPort(port uint64) *BlackboxExporterScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of BlackboxExporterScript
func (c *BlackboxExporterScript) WithNumaNode(numa string) *BlackboxExporterScript {
	c.NumaNode = numa
	return c
}

// Config generate the config file data.
func (c *BlackboxExporterScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_blackbox_exporter.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *BlackboxExporterScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the BlackboxExporter config content by tpl
func (c *BlackboxExporterScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("BlackboxExporter").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
