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

// PumpScript represent the data to generate Pump config
type PumpScript struct {
	NodeID    string
	Host      string
	Port      int
	DeployDir string
	DataDir   string
	LogDir    string
	NumaNode  string
	CommitTs  int64
	Endpoints []*PDScript
}

// NewPumpScript returns a PumpScript with given arguments
func NewPumpScript(nodeID, host, deployDir, dataDir, logDir string) *PumpScript {
	return &PumpScript{
		NodeID:    nodeID,
		Host:      host,
		Port:      8250,
		DeployDir: deployDir,
		DataDir:   dataDir,
		LogDir:    logDir,
	}
}

// WithPort set Port field of PumpScript
func (c *PumpScript) WithPort(port int) *PumpScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of PumpScript
func (c *PumpScript) WithNumaNode(numa string) *PumpScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new PumpScript to Endpoints field
func (c *PumpScript) AppendEndpoints(ends ...*PDScript) *PumpScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config generate the config file data.
func (c *PumpScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_pump.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific file.
func (c *PumpScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the Pump config content by tpl
func (c *PumpScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Pump").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
