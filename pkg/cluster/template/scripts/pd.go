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
	"errors"
	"io/ioutil"
	"path"
	"text/template"

	"github.com/pingcap/tiup/pkg/cluster/embed"
	"github.com/pingcap/tiup/pkg/logger/log"
)

// PDScript represent the data to generate pd config
type PDScript struct {
	Name       string
	Scheme     string
	IP         string
	ListenHost string
	ClientPort int
	PeerPort   int
	DeployDir  string
	DataDir    string
	LogDir     string
	NumaNode   string
	Endpoints  []*PDScript
}

// NewPDScript returns a PDScript with given arguments
func NewPDScript(name, ip, deployDir, dataDir, logDir string) *PDScript {
	return &PDScript{
		Name:       name,
		Scheme:     "http",
		IP:         ip,
		ClientPort: 2379,
		PeerPort:   2380,
		DeployDir:  deployDir,
		DataDir:    dataDir,
		LogDir:     logDir,
	}
}

// WithListenHost set listenHost field of PDScript
func (c *PDScript) WithListenHost(listenHost string) *PDScript {
	c.ListenHost = listenHost
	return c
}

// WithScheme set Scheme field of PDScript
func (c *PDScript) WithScheme(scheme string) *PDScript {
	c.Scheme = scheme
	return c
}

// WithClientPort set ClientPort field of PDScript
func (c *PDScript) WithClientPort(port int) *PDScript {
	c.ClientPort = port
	return c
}

// WithPeerPort set PeerPort field of PDScript
func (c *PDScript) WithPeerPort(port int) *PDScript {
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

// Config generate the config file data.
func (c *PDScript) Config() ([]byte, error) {
	fp := path.Join("/templates", "scripts", "run_pd.sh.tpl")
	tpl, err := embed.ReadFile(fp)
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

	if c.Name == "" {
		return nil, errors.New("empty name")
	}
	for _, s := range c.Endpoints {
		if s.Name == "" {
			return nil, errors.New("empty name")
		}
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// PDScaleScript represent the data to generate pd config on scaling
type PDScaleScript struct {
	PDScript
}

// NewPDScaleScript return a new PDScaleScript
func NewPDScaleScript(name, ip, deployDir, dataDir, logDir string) *PDScaleScript {
	return &PDScaleScript{*NewPDScript(name, ip, deployDir, dataDir, logDir)}
}

// WithScheme set Scheme field of PDScaleScript
func (c *PDScaleScript) WithScheme(scheme string) *PDScaleScript {
	c.Scheme = scheme
	return c
}

// WithClientPort set ClientPort field of PDScaleScript
func (c *PDScaleScript) WithClientPort(port int) *PDScaleScript {
	c.ClientPort = port
	return c
}

// WithPeerPort set PeerPort field of PDScript
func (c *PDScaleScript) WithPeerPort(port int) *PDScaleScript {
	c.PeerPort = port
	return c
}

// WithNumaNode set NumaNode field of PDScaleScript
func (c *PDScaleScript) WithNumaNode(numa string) *PDScaleScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new PDScaleScript to Endpoints field
func (c *PDScaleScript) AppendEndpoints(ends ...*PDScript) *PDScaleScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config generate the config file data.
func (c *PDScaleScript) Config() ([]byte, error) {
	fp := path.Join("/templates", "scripts", "run_pd_scale.sh.tpl")
	log.Infof("script path: %s", fp)
	tpl, err := embed.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *PDScaleScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}
