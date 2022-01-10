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

// DrainerScript represent the data to generate drainer config
type DrainerScript struct {
	NodeID    string
	IP        string
	Port      int
	DeployDir string
	DataDir   string
	LogDir    string
	NumaNode  string
	Endpoints []*PDScript
}

// NewDrainerScript returns a DrainerScript with given arguments
func NewDrainerScript(nodeID, ip, deployDir, dataDir, logDir string) *DrainerScript {
	return &DrainerScript{
		NodeID:    nodeID,
		IP:        ip,
		Port:      8249,
		DeployDir: deployDir,
		DataDir:   dataDir,
		LogDir:    logDir,
	}
}

// WithPort set Port field of DrainerScript
func (c *DrainerScript) WithPort(port int) *DrainerScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of DrainerScript
func (c *DrainerScript) WithNumaNode(numa string) *DrainerScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new DrainerScript to Endpoints field
func (c *DrainerScript) AppendEndpoints(ends ...*PDScript) *DrainerScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config generate the config file data.
func (c *DrainerScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_drainer.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific file.
func (c *DrainerScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the Drainer config content by tpl
func (c *DrainerScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Drainer").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
