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

// PumpConfig represent the data to generate Pump config
type PumpConfig struct{}

// NewPumpConfig returns a PumpConfig
func NewPumpConfig() *PumpConfig {
	return &PumpConfig{}
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/config/pump.yml
// and generate the config by ConfigWithTemplate
func (c *PumpConfig) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "config", "pump.toml")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *PumpConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the Pump config content by tpl
func (c *PumpConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	return []byte(tpl), nil
}
