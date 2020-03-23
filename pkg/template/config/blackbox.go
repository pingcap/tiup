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

// BlackboxConfig represent the data to generate AlertManager config
type BlackboxConfig struct{}

// NewBlackboxConfig returns a BlackboxConfig
func NewBlackboxConfig() *BlackboxConfig {
	return &BlackboxConfig{}
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/config/alertmanager.yml
// and generate the config by ConfigWithTemplate
func (c *BlackboxConfig) Config() (string, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "config", "blackbox.yml")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return "", err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the AlertManager config content by tpl
func (c *BlackboxConfig) ConfigWithTemplate(tpl string) (string, error) {
	return tpl, nil
}
