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
	"github.com/pingcap/tiup/pkg/utils"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pingcap/tiup/embed"
)

// TiSparkConfig represent the data to generate TiSpark configs
type TiSparkConfig struct {
	TiSparkMasters string
	CustomFields   map[string]interface{}
	Endpoints      []string
	tiSparkVersion string
}

// NewTiSparkConfig returns a TiSparkConfig
func NewTiSparkConfig(pds []string) *TiSparkConfig {
	return &TiSparkConfig{Endpoints: pds}
}

// WithMasters sets master address
func (c *TiSparkConfig) WithMasters(masters string) *TiSparkConfig {
	c.TiSparkMasters = masters
	return c
}

// WithCustomFields sets custom setting fields
func (c *TiSparkConfig) WithCustomFields(m map[string]interface{}) *TiSparkConfig {
	c.CustomFields = m
	return c
}

// WithTiSparkVersion sets TiSpark version
func (c *TiSparkConfig) WithTiSparkVersion(version string) *TiSparkConfig {
	c.tiSparkVersion = version
	return c
}

// Config generate the config file data.
func (c *TiSparkConfig) Config() ([]byte, error) {
	// After TiSpark 2.5.0(include), we need to contain catalog configs
	const tiSparkVersionForCatalog = utils.Version("v2.5.0")
	var fp string
	if c.tiSparkVersion != "" && utils.Version(c.tiSparkVersion) < tiSparkVersionForCatalog {
		fp = filepath.Join("templates", "config", "spark-defaults.conf.tpl")
	} else {
		fp = filepath.Join("templates", "config", "spark3-defaults.conf.tpl")
	}
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *TiSparkConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate parses the template file
func (c *TiSparkConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("TiSpark").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
