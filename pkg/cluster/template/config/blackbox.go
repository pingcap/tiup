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
	"io/ioutil"
	"path"

	"github.com/pingcap/tiup/embed"
)

// BlackboxConfig represent the data to generate AlertManager config
type BlackboxConfig struct{}

// NewBlackboxConfig returns a BlackboxConfig
func NewBlackboxConfig() *BlackboxConfig {
	return &BlackboxConfig{}
}

// Config generate the config file data.
func (c *BlackboxConfig) Config() ([]byte, error) {
	fp := path.Join("/templates", "config", "blackbox.yml")
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
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the AlertManager config content by tpl
func (c *BlackboxConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	return []byte(tpl), nil
}
