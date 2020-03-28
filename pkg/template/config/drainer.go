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

// DrainerConfig represent the data to generate Drainer config
type DrainerConfig struct{}

// NewDrainerConfig returns a DrainerConfig
func NewDrainerConfig() *DrainerConfig {
	return &DrainerConfig{}
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/config/drainer.yml
// and generate the config by ConfigWithTemplate
func (c *DrainerConfig) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "config", "drainer.toml")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *DrainerConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the Drainer config content by tpl
func (c *DrainerConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	return []byte(tpl), nil
}
