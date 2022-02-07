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
	"os"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// DMMasterScript represent the data to generate TiDB config
type DMMasterScript struct {
	Name         string
	Scheme       string
	IP           string
	Port         int
	PeerPort     int
	DeployDir    string
	DataDir      string
	LogDir       string
	NumaNode     string
	V1SourcePath string
	Endpoints    []*DMMasterScript
}

// NewDMMasterScript returns a DMMasterScript with given arguments
func NewDMMasterScript(name, ip, deployDir, dataDir, logDir string, enableTLS bool) *DMMasterScript {
	return &DMMasterScript{
		Name:      name,
		Scheme:    utils.Ternary(enableTLS, "https", "http").(string),
		IP:        ip,
		Port:      8261,
		PeerPort:  8291,
		DeployDir: deployDir,
		DataDir:   dataDir,
		LogDir:    logDir,
	}
}

// WithV1SourcePath set Scheme field of V1SourcePath
func (c *DMMasterScript) WithV1SourcePath(path string) *DMMasterScript {
	c.V1SourcePath = path
	return c
}

// WithScheme set Scheme field of NewDMMasterScript
func (c *DMMasterScript) WithScheme(scheme string) *DMMasterScript {
	c.Scheme = scheme
	return c
}

// WithPort set Port field of DMMasterScript
func (c *DMMasterScript) WithPort(port int) *DMMasterScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of DMMasterScript
func (c *DMMasterScript) WithNumaNode(numa string) *DMMasterScript {
	c.NumaNode = numa
	return c
}

// WithPeerPort set PeerPort field of DMMasterScript
func (c *DMMasterScript) WithPeerPort(port int) *DMMasterScript {
	c.PeerPort = port
	return c
}

// AppendEndpoints add new DMMasterScript to Endpoints field
func (c *DMMasterScript) AppendEndpoints(ends ...*DMMasterScript) *DMMasterScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config generate the config file data.
func (c *DMMasterScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_dm-master.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *DMMasterScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the TiDB config content by tpl
func (c *DMMasterScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("dm-master").Parse(tpl)
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

// DMMasterScaleScript represent the data to generate dm-master config on scaling
type DMMasterScaleScript struct {
	DMMasterScript
}

// NewDMMasterScaleScript return a new DMMasterScaleScript
func NewDMMasterScaleScript(name, ip, deployDir, dataDir, logDir string, enableTLS bool) *DMMasterScaleScript {
	return &DMMasterScaleScript{*NewDMMasterScript(name, ip, deployDir, dataDir, logDir, enableTLS)}
}

// WithScheme set Scheme field of DMMasterScaleScript
func (c *DMMasterScaleScript) WithScheme(scheme string) *DMMasterScaleScript {
	c.Scheme = scheme
	return c
}

// WithPort set Port field of DMMasterScript
func (c *DMMasterScaleScript) WithPort(port int) *DMMasterScaleScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of DMMasterScript
func (c *DMMasterScaleScript) WithNumaNode(numa string) *DMMasterScaleScript {
	c.NumaNode = numa
	return c
}

// WithPeerPort set PeerPort field of DMMasterScript
func (c *DMMasterScaleScript) WithPeerPort(port int) *DMMasterScaleScript {
	c.PeerPort = port
	return c
}

// AppendEndpoints add new DMMasterScript to Endpoints field
func (c *DMMasterScaleScript) AppendEndpoints(ends ...*DMMasterScript) *DMMasterScaleScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config generate the config file data.
func (c *DMMasterScaleScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_dm-master_scale.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *DMMasterScaleScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}
