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

// TiFlashScript represent the data to generate TiFlash config
type TiFlashScript struct {
	IP                   string
	TCPPort              int
	HTTPPort             int
	FlashServicePort     int
	FlashProxyPort       int
	FlashProxyStatusPort int
	StatusPort           int
	DeployDir            string
	DataDir              string
	LogDir               string
	NumaNode             string
	Endpoints            []*PDScript
	TiDBStatusAddrs      string
	PDAddrs              string
}

// NewTiFlashScript returns a TiFlashScript with given arguments
func NewTiFlashScript(ip, deployDir, dataDir string, logDir string, tidbStatusAddrs string, pdAddrs string) *TiFlashScript {
	return &TiFlashScript{
		IP:                   ip,
		TCPPort:              9000,
		HTTPPort:             8123,
		FlashServicePort:     3930,
		FlashProxyPort:       20170,
		FlashProxyStatusPort: 20292,
		StatusPort:           8234,
		DeployDir:            deployDir,
		DataDir:              dataDir,
		LogDir:               logDir,
		TiDBStatusAddrs:      tidbStatusAddrs,
		PDAddrs:              pdAddrs,
	}
}

// WithTCPPort set TCPPort field of TiFlashScript
func (c *TiFlashScript) WithTCPPort(port int) *TiFlashScript {
	c.TCPPort = port
	return c
}

// WithHTTPPort set HTTPPort field of TiFlashScript
func (c *TiFlashScript) WithHTTPPort(port int) *TiFlashScript {
	c.HTTPPort = port
	return c
}

// WithFlashServicePort set FlashServicePort field of TiFlashScript
func (c *TiFlashScript) WithFlashServicePort(port int) *TiFlashScript {
	c.FlashServicePort = port
	return c
}

// WithFlashProxyPort set FlashProxyPort field of TiFlashScript
func (c *TiFlashScript) WithFlashProxyPort(port int) *TiFlashScript {
	c.FlashProxyPort = port
	return c
}

// WithFlashProxyStatusPort set FlashProxyStatusPort field of TiFlashScript
func (c *TiFlashScript) WithFlashProxyStatusPort(port int) *TiFlashScript {
	c.FlashProxyStatusPort = port
	return c
}

// WithStatusPort set FlashProxyStatusPort field of TiFlashScript
func (c *TiFlashScript) WithStatusPort(port int) *TiFlashScript {
	c.StatusPort = port
	return c
}

// WithNumaNode set NumaNode field of TiFlashScript
func (c *TiFlashScript) WithNumaNode(numa string) *TiFlashScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new PDScript to Endpoints field
func (c *TiFlashScript) AppendEndpoints(ends ...*PDScript) *TiFlashScript {
	c.Endpoints = append(c.Endpoints, ends...)
	return c
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/scripts/run_TiFlash.sh.tpl as template
// and generate the config by ConfigWithTemplate
func (c *TiFlashScript) Config() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "scripts", "run_tiflash.sh.tpl")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigToFile write config content to specific path
func (c *TiFlashScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigTiFlashLearner read ${localdata.EnvNameComponentInstallDir}/templates/conf/tiflash-learner.toml.tpl as template
// and generate the config by ConfigWithTemplate
func (c *TiFlashScript) ConfigTiFlashLearner() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "config", "tiflash-learner.toml.tpl")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigTiFlashLearnerToFile write config content to specific path
func (c *TiFlashScript) ConfigTiFlashLearnerToFile(file string) error {
	config, err := c.ConfigTiFlashLearner()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigTiFlash read ${localdata.EnvNameComponentInstallDir}/templates/conf/tiflash.toml.tpl as template
// and generate the config by ConfigWithTemplate
func (c *TiFlashScript) ConfigTiFlash() ([]byte, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "config", "tiflash.toml.tpl")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigTiFlashToFile write config content to specific path
func (c *TiFlashScript) ConfigTiFlashToFile(file string) error {
	config, err := c.ConfigTiFlash()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, config, 0755)
}

// ConfigWithTemplate generate the TiFlash config content by tpl
func (c *TiFlashScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("TiFlash").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
