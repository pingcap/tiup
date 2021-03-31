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

package scripts

import (
	"bytes"
	"os"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
)

// CDCScript represent the data to generate cdc config
type CDCScript struct {
	IP                string
	Port              int
	DeployDir         string
	LogDir            string
	NumaNode          string
	GCTTL             int64
	TZ                string
	TLSEnabled        bool
	Endpoints         []*PDScript
	ConfigFileEnabled bool
}

// NewCDCScript returns a CDCScript with given arguments
func NewCDCScript(ip, deployDir, logDir string, enableTLS bool, gcTTL int64, tz string) *CDCScript {
	return &CDCScript{
		IP:         ip,
		Port:       8300,
		DeployDir:  deployDir,
		LogDir:     logDir,
		TLSEnabled: enableTLS,
		GCTTL:      gcTTL,
		TZ:         tz,
	}
}

// WithPort set Port field of TiCDCScript
func (c *CDCScript) WithPort(port int) *CDCScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of TiCDCScript
func (c *CDCScript) WithNumaNode(numa string) *CDCScript {
	c.NumaNode = numa
	return c
}

// WithConfigFileEnabled enables config file
func (c *CDCScript) WithConfigFileEnabled() *CDCScript {
	c.ConfigFileEnabled = true
	return c
}

// Config generate the config file data.
func (c *CDCScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_cdc.sh.tpl")
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific file.
func (c *CDCScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the CDC config content by tpl
func (c *CDCScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("CDC").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// AppendEndpoints add new PDScript to Endpoints field
func (c *CDCScript) AppendEndpoints(ends ...*PDScript) *CDCScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}
