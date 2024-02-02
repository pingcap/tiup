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
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// Config represent the data to generate systemd config
type Config struct {
	ServiceName         string
	User                string
	MemoryLimit         string
	CPUQuota            string
	IOReadBandwidthMax  string
	IOWriteBandwidthMax string
	LimitCORE           string
	DeployDir           string
	DisableSendSigkill  bool
	GrantCapNetRaw      bool
	// Takes one of no, on-success, on-failure, on-abnormal, on-watchdog, on-abort, or always.
	// The Template set as always if this is not setted.
	Restart     string
	SystemdMode string
}

// NewConfig returns a Config with given arguments
func NewConfig(service, user, deployDir string) *Config {
	return &Config{
		ServiceName: service,
		User:        user,
		DeployDir:   deployDir,
	}
}

// WithMemoryLimit set the MemoryLimit field of Config
func (c *Config) WithMemoryLimit(mem string) *Config {
	c.MemoryLimit = mem
	return c
}

// WithCPUQuota set the CPUQuota field of Config
func (c *Config) WithCPUQuota(cpu string) *Config {
	c.CPUQuota = cpu
	return c
}

// WithIOReadBandwidthMax set the IOReadBandwidthMax field of Config
func (c *Config) WithIOReadBandwidthMax(io string) *Config {
	c.IOReadBandwidthMax = io
	return c
}

// WithIOWriteBandwidthMax set the IOWriteBandwidthMax field of Config
func (c *Config) WithIOWriteBandwidthMax(io string) *Config {
	c.IOWriteBandwidthMax = io
	return c
}

// WithLimitCORE set the LimitCORE field of Config
func (c *Config) WithLimitCORE(core string) *Config {
	c.LimitCORE = core
	return c
}

// WithSystemdMode set the SystemdMode field of Config
func (c *Config) WithSystemdMode(mode string) *Config {
	c.SystemdMode = mode
	return c
}

// ConfigToFile write config content to specific path
func (c *Config) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, config, 0755)
}

// Config generate the config file data.
func (c *Config) Config() ([]byte, error) {
	fp := path.Join("templates", "systemd", "system.service.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the system config content by tpl
func (c *Config) ConfigWithTemplate(tpl string) ([]byte, error) {
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
