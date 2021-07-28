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

// DMWorkerScript represent the data to generate TiDB config
type DMWorkerScript struct {
	Name      string
	IP        string
	Port      int
	DeployDir string
	LogDir    string
	NumaNode  string
	Endpoints []*DMMasterScript
}

// NewDMWorkerScript returns a DMWorkerScript with given arguments
func NewDMWorkerScript(name, ip, deployDir, logDir string) *DMWorkerScript {
	return &DMWorkerScript{
		Name:      name,
		IP:        ip,
		Port:      8262,
		DeployDir: deployDir,
		LogDir:    logDir,
	}
}

// WithPort set Port field of DMWorkerScript
func (c *DMWorkerScript) WithPort(port int) *DMWorkerScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of DMWorkerScript
func (c *DMWorkerScript) WithNumaNode(numa string) *DMWorkerScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new PDScript to Endpoints field
func (c *DMWorkerScript) AppendEndpoints(ends ...*DMMasterScript) *DMWorkerScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config generate the config file data.
func (c *DMWorkerScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_dm-worker.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *DMWorkerScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the DM worker config content by tpl
func (c *DMWorkerScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("dm-worker").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
