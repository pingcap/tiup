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

// PDScript represent the data to generate pd config
type PDScript struct {
	Name       string
	Scheme     string
	IP         string
	ClientPort uint64
	PeerPort   uint64
	DeployDir  string
	DataDir    string
	NumaNode   string
	Endpoints  []*PDScript
}

// NewPDScript returns a PDScript with given arguments
func NewPDScript(name, ip, deployDir, dataDir string) *PDScript {
	return &PDScript{
		Name:       name,
		Scheme:     "http",
		IP:         ip,
		ClientPort: 2379,
		PeerPort:   2380,
		DeployDir:  deployDir,
		DataDir:    dataDir,
	}
}

// WithScheme set Scheme field of PDScript
func (c *PDScript) WithScheme(scheme string) *PDScript {
	c.Scheme = scheme
	return c
}

// WithClientPort set ClientPort field of PDScript
func (c *PDScript) WithClientPort(port uint64) *PDScript {
	c.ClientPort = port
	return c
}

// WithPeerPort set PeerPort field of PDScript
func (c *PDScript) WithPeerPort(port uint64) *PDScript {
	c.PeerPort = port
	return c
}

// WithNumaNode set NumaNode field of PDScript
func (c *PDScript) WithNumaNode(numa string) *PDScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new PDScript to Endpoints field
func (c *PDScript) AppendEndpoints(ends ...*PDScript) *PDScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/scripts/run_pd.sh.tpl as template
// and generate the config by ConfigWithTemplate
func (c *PDScript) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "scripts", "run_pd.sh.tpl")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *PDScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the PD config content by tpl
func (c *PDScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("PD").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
