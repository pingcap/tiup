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

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/embed"
)

// AlertManagerConfig represent the data to generate AlertManager config
type AlertManagerConfig struct{}

// NewAlertManagerConfig returns a AlertManagerConfig
func NewAlertManagerConfig() *AlertManagerConfig {
	return &AlertManagerConfig{}
}

// Config generate the config file data.
func (c *AlertManagerConfig) Config() ([]byte, error) {
	fp := path.Join("/templates", "config", "alertmanager.yml")
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the AlertManager config content by tpl
func (c *AlertManagerConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	return []byte(tpl), nil
}

// ConfigToFile write config content to specific path
func (c *AlertManagerConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(file, config, 0755); err != nil {
		return errors.AddStack(err)
	}
	return nil
}
