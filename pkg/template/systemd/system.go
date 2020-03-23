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

package system

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
)

// SystemConfig represent the data to generate systemd config
type SystemConfig struct {
	ServiceName         string
	User                string
	MemoryLimit         uint
	CPUQuota            uint
	IOReadBandwidthMax  uint
	IOWriteBandwidthMax uint
	DeployDir           string
}

// NewSystemConfig returns a SystemConfig with given arguments
func NewSystemConfig(service, user, deployDir string) *SystemConfig {
	return &SystemConfig{
		ServiceName: service,
		User:        user,
		DeployDir:   deployDir,
	}
}

// WithMemoryLimit set the MemoryLimit field of SystemConfig
func (c *SystemConfig) WithMemoryLimit(mem uint) *SystemConfig {
	c.MemoryLimit = mem
	return c
}

// WithCPUQuota set the CPUQuota field of SystemConfig
func (c *SystemConfig) WithCPUQuota(cpu uint) *SystemConfig {
	c.CPUQuota = cpu
	return c
}

// WithIOReadBandwidthMax set the IOReadBandwidthMax field of SystemConfig
func (c *SystemConfig) WithIOReadBandwidthMax(io uint) *SystemConfig {
	c.IOReadBandwidthMax = io
	return c
}

// WithIOWriteBandwidthMax set the IOWriteBandwidthMax field of SystemConfig
func (c *SystemConfig) WithIOWriteBandwidthMax(io uint) *SystemConfig {
	c.IOWriteBandwidthMax = io
	return c
}

// ConfigToFile write config content to specific path
func (c *SystemConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/systemd/system.service.tpl as template
// and generate the config by ConfigWithTemplate
func (c *SystemConfig) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "systemd", "system.service.tpl")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the system config content by tpl
func (c *SystemConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("system").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
