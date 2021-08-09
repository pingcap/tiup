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

// BlackboxConfig represent the data to generate AlertManager config
type BlackboxConfig struct {
	DeployDir  string
	TLSEnabled bool
}

// NewBlackboxConfig returns a BlackboxConfig
func NewBlackboxConfig(deployDir string, tlsEnabled bool) *BlackboxConfig {
	return &BlackboxConfig{
		DeployDir:  deployDir,
		TLSEnabled: tlsEnabled,
	}
}

// Config generate the config file data.
func (c *BlackboxConfig) Config() ([]byte, error) {
	fp := path.Join("templates", "config", "blackbox.yml.tpl")
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *BlackboxConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the AlertManager config content by tpl
func (c *BlackboxConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Blackbox").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
