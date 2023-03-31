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
	"strings"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiSparkConfig represent the data to generate systemd config
type TiSparkConfig struct {
	ServiceName string
	User        string
	DeployDir   string
	JavaHome    string
	// Takes one of no, on-success, on-failure, on-abnormal, on-watchdog, on-abort, or always.
	// The Template set as always if this is not setted.
	Restart string
}

// NewTiSparkConfig returns a Config with given arguments
func NewTiSparkConfig(service, user, deployDir, javaHome string) *TiSparkConfig {
	if strings.Contains(service, "master") {
		service = "master"
	} else if strings.Contains(service, "worker") {
		service = "slave"
	}
	return &TiSparkConfig{
		ServiceName: service,
		User:        user,
		DeployDir:   deployDir,
		JavaHome:    javaHome,
	}
}

// ConfigToFile write config content to specific path
func (c *TiSparkConfig) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, config, 0755)
}

// Config generate the config file data.
func (c *TiSparkConfig) Config() ([]byte, error) {
	fp := path.Join("templates", "systemd", "tispark.service.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the system config content by tpl
func (c *TiSparkConfig) ConfigWithTemplate(tpl string) ([]byte, error) {
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
