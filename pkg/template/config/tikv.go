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
	"os"
	"path"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
)

// TiKVConfig represent the data to generate TiKV config
type TiKVConfig struct{}

// NewTiKVConfig returns a TiKVConfig
func NewTiKVConfig() *TiKVConfig {
	return &TiKVConfig{}
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/config/TiKV.yml
// and generate the config by ConfigWithTemplate
func (c *TiKVConfig) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "config", "tikv.toml")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *TiKVConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the TiKV config content by tpl
func (c *TiKVConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	return []byte(tpl), nil
}
