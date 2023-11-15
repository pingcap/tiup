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
	"path/filepath"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiSparkConfig represent the data to generate TiSpark configs
type TiSparkConfig struct {
	TiSparkMasters string
	CustomFields   map[string]any
	Endpoints      []string
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
func (c *TiSparkConfig) WithCustomFields(m map[string]any) *TiSparkConfig {
	c.CustomFields = m
	return c
}

// Config generate the config file data.
func (c *TiSparkConfig) Config() ([]byte, error) {
	fp := filepath.Join("templates", "config", "spark-defaults.conf.tpl")
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
	return utils.WriteFile(file, config, 0755)
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
