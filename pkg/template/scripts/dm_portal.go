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
	"io/ioutil"
	"path"
	"text/template"

	"github.com/pingcap-incubator/tiup-cluster/pkg/embed"
)

// DMPortalScript represent the data to generate dm portal config
type DMPortalScript struct {
	Name      string
	IP        string
	Port      int
	DataDir   string
	DeployDir string
	LogDir    string
	NumaNode  string
	Timeout   int
}

// NewDMPortalScript returns a DMWorkerScript with given arguments
func NewDMPortalScript(ip, deployDir, dataDir, logDir string) *DMPortalScript {
	return &DMPortalScript{
		IP:        ip,
		Port:      8280,
		DeployDir: deployDir,
		DataDir:   dataDir,
		LogDir:    logDir,
		Timeout:   5,
	}
}

// WithPort set Port field of DMWorkerScript
func (c *DMPortalScript) WithPort(port int) *DMPortalScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of DMWorkerScript
func (c *DMPortalScript) WithNumaNode(numa string) *DMPortalScript {
	c.NumaNode = numa
	return c
}

// WithTimeout set Timeout field of DMWorkerScript
func (c *DMPortalScript) WithTimeout(timeout int) *DMPortalScript {
	c.Timeout = timeout
	return c
}

// Config generate the config file data.
func (c *DMPortalScript) Config() ([]byte, error) {
	fp := path.Join("/templates", "scripts", "run_dm-portal.sh.tpl")
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *DMPortalScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the DM worker config content by tpl
func (c *DMPortalScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("dm-portal").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
