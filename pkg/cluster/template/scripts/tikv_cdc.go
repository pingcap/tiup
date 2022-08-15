// Copyright 2022 PingCAP, Inc.
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

// TiKVCDCScript represent the data to generate cdc config
type TiKVCDCScript struct {
	IP         string
	Port       int
	DeployDir  string
	LogDir     string
	DataDir    string
	NumaNode   string
	GCTTL      int64
	TZ         string
	TLSEnabled bool
	Endpoints  []*PDScript
}

// NewTiKVCDCScript returns a TiKVCDCScript with given arguments
func NewTiKVCDCScript(ip, deployDir, logDir, dataDir string, enableTLS bool, gcTTL int64, tz string) *TiKVCDCScript {
	return &TiKVCDCScript{
		IP:         ip,
		Port:       8600,
		DeployDir:  deployDir,
		LogDir:     logDir,
		DataDir:    dataDir,
		TLSEnabled: enableTLS,
		GCTTL:      gcTTL,
		TZ:         tz,
	}
}

// WithPort set Port field of TiKVCDCScript
func (c *TiKVCDCScript) WithPort(port int) *TiKVCDCScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of TiKVCDCScript
func (c *TiKVCDCScript) WithNumaNode(numa string) *TiKVCDCScript {
	c.NumaNode = numa
	return c
}

// Config generate the config file data.
func (c *TiKVCDCScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_tikv-cdc.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific file.
func (c *TiKVCDCScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the TiKV-CDC config content by tpl
func (c *TiKVCDCScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("TiKVCDC").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// AppendEndpoints add new PDScript to Endpoints field
func (c *TiKVCDCScript) AppendEndpoints(ends ...*PDScript) *TiKVCDCScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}
