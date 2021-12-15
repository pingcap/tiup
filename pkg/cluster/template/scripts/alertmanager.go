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

// AlertManagerScript represent the data to generate AlertManager start script
type AlertManagerScript struct {
	IP          string
	ListenHost  string
	WebPort     int
	ClusterPort int
	DeployDir   string
	DataDir     string
	LogDir      string
	NumaNode    string
	TLSEnabled  bool
	EndPoints   []*AlertManagerScript
}

// NewAlertManagerScript returns a AlertManagerScript with given arguments
func NewAlertManagerScript(ip, listenHost, deployDir, dataDir, logDir string, enableTLS bool) *AlertManagerScript {
	if listenHost == "" {
		listenHost = ip
	}
	return &AlertManagerScript{
		IP:          ip,
		ListenHost:  listenHost,
		WebPort:     9093,
		ClusterPort: 9094,
		DeployDir:   deployDir,
		DataDir:     dataDir,
		LogDir:      logDir,
		TLSEnabled:  enableTLS,
	}
}

// WithWebPort set WebPort field of AlertManagerScript
func (c *AlertManagerScript) WithWebPort(port int) *AlertManagerScript {
	c.WebPort = port
	return c
}

// WithClusterPort set WebPort field of AlertManagerScript
func (c *AlertManagerScript) WithClusterPort(port int) *AlertManagerScript {
	c.ClusterPort = port
	return c
}

// WithNumaNode set NumaNode field of AlertManagerScript
func (c *AlertManagerScript) WithNumaNode(numa string) *AlertManagerScript {
	c.NumaNode = numa
	return c
}

// AppendEndpoints add new alert manager to Endpoints field
func (c *AlertManagerScript) AppendEndpoints(ends []*AlertManagerScript) *AlertManagerScript {
	c.EndPoints = append(c.EndPoints, ends...)
	return c
}

// ConfigToFile write config content to specific path
func (c *AlertManagerScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}

// Config generate the config file data.
func (c *AlertManagerScript) Config() ([]byte, error) {
	fp := path.Join("templates", "scripts", "run_alertmanager.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the AlertManager config content by tpl
func (c *AlertManagerScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("AlertManager").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
