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
	"os"
	"path"
	"text/template"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
)

// TiKVScript represent the data to generate TiKV config
type TiKVScript struct {
	IP         string
	Port       int
	StatusPort int
	DeployDir  string
	DataDir    string
	LogDir     string
	NumaNode   string
	Endpoints  []*PDScript
}

// NewTiKVScript returns a TiKVScript with given arguments
func NewTiKVScript(ip, deployDir, dataDir, logDir string) *TiKVScript {
	return &TiKVScript{
		IP:         ip,
		Port:       20160,
		StatusPort: 20180,
		DeployDir:  deployDir,
		DataDir:    dataDir,
		LogDir:     logDir,
	}
}

// WithPort set Port field of TiKVScript
func (c *TiKVScript) WithPort(port int) *TiKVScript {
	c.Port = port
	return c
}

// WithStatusPort set StatusPort field of TiKVScript
func (c *TiKVScript) WithStatusPort(port int) *TiKVScript {
	c.StatusPort = port
	return c
}

// WithNumaNode set NumaNode field of TiKVScript
func (c *TiKVScript) WithNumaNode(numa string) *TiKVScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new PDScript to Endpoints field
func (c *TiKVScript) AppendEndpoints(ends ...*PDScript) *TiKVScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/scripts/run_TiKV.sh.tpl as template
// and generate the config by ConfigWithTemplate
func (c *TiKVScript) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "scripts", "run_tikv.sh.tpl")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *TiKVScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the TiKV config content by tpl
func (c *TiKVScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("TiKV").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
