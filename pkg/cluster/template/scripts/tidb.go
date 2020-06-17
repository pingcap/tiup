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

	"github.com/pingcap/tiup/pkg/cluster/embed"
)

// TiDBScript represent the data to generate TiDB config
type TiDBScript struct {
	IP            string
	ListenHost string
	Port          int
	StatusPort    int
	DeployDir     string
	LogDir        string
	NumaNode      string
	Endpoints     []*PDScript
}

// NewTiDBScript returns a TiDBScript with given arguments
func NewTiDBScript(ip, deployDir, logDir string) *TiDBScript {
	return &TiDBScript{
		IP:         ip,
		Port:       4000,
		StatusPort: 10080,
		DeployDir:  deployDir,
		LogDir:     logDir,
	}
}

// WithListenHost set ListenHost field of TiDBScript
func (c *TiDBScript) WithListenHost(listenHost string) *TiDBScript {
	c.ListenHost = listenHost
	return c
}

// WithPort set Port field of TiDBScript
func (c *TiDBScript) WithPort(port int) *TiDBScript {
	c.Port = port
	return c
}

// WithStatusPort set StatusPort field of TiDBScript
func (c *TiDBScript) WithStatusPort(port int) *TiDBScript {
	c.StatusPort = port
	return c
}

// WithNumaNode set NumaNode field of TiDBScript
func (c *TiDBScript) WithNumaNode(numa string) *TiDBScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new PDScript to Endpoints field
func (c *TiDBScript) AppendEndpoints(ends ...*PDScript) *TiDBScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config generate the config file data.
func (c *TiDBScript) Config() ([]byte, error) {
	fp := path.Join("/templates", "scripts", "run_tidb.sh.tpl")
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *TiDBScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the TiDB config content by tpl
func (c *TiDBScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("TiDB").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
