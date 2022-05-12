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
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/tidbver"
)

// TiKVScript represent the data to generate TiKV config
type TiKVScript struct {
	IP                         string
	ListenHost                 string
	AdvertiseAddr              string
	AdvertiseStatusAddr        string
	Port                       int
	StatusPort                 int
	DeployDir                  string
	DataDir                    string
	LogDir                     string
	SupportAdvertiseStatusAddr bool
	NumaNode                   string
	Endpoints                  []*PDScript
}

// NewTiKVScript returns a TiKVScript with given arguments
func NewTiKVScript(version, ip string, port, statusPort int, deployDir, dataDir, logDir string) *TiKVScript {
	return &TiKVScript{
		IP:                         ip,
		AdvertiseAddr:              fmt.Sprintf("%s:%d", ip, port),
		AdvertiseStatusAddr:        fmt.Sprintf("%s:%d", ip, statusPort),
		Port:                       port,
		StatusPort:                 statusPort,
		DeployDir:                  deployDir,
		DataDir:                    dataDir,
		LogDir:                     logDir,
		SupportAdvertiseStatusAddr: tidbver.TiKVSupportAdvertiseStatusAddr(version),
	}
}

// WithListenHost set ListenHost field of TiKVScript
func (c *TiKVScript) WithListenHost(listenHost string) *TiKVScript {
	c.ListenHost = listenHost
	return c
}

// WithAdvertiseAddr set AdvertiseAddr of TiKVScript
func (c *TiKVScript) WithAdvertiseAddr(addr string) *TiKVScript {
	if addr != "" {
		c.AdvertiseAddr = addr
	}
	return c
}

// WithAdvertiseStatusAddr set AdvertiseStatusAddr of TiKVScript
func (c *TiKVScript) WithAdvertiseStatusAddr(addr string) *TiKVScript {
	if addr != "" {
		c.AdvertiseStatusAddr = addr
	}
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

// Config generate the config file data.
func (c *TiKVScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_tikv.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
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
	return os.WriteFile(file, config, 0755)
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
