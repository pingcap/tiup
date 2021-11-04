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
	"regexp"
	"text/template"

	"github.com/pingcap/tiup/embed"
)

// PrometheusScript represent the data to generate Prometheus config
type PrometheusScript struct {
	IP        string
	Port      int
	DeployDir string
	DataDir   string
	LogDir    string
	NumaNode  string
	Retention string
	tplFile   string
	EnableNG  bool
}

// NewPrometheusScript returns a PrometheusScript with given arguments
func NewPrometheusScript(ip, deployDir, dataDir, logDir string) *PrometheusScript {
	return &PrometheusScript{
		IP:        ip,
		Port:      9090,
		DeployDir: deployDir,
		DataDir:   dataDir,
		LogDir:    logDir,
	}
}

// WithPort set Port field of PrometheusScript
func (c *PrometheusScript) WithPort(port int) *PrometheusScript {
	c.Port = port
	return c
}

// WithNumaNode set NumaNode field of PrometheusScript
func (c *PrometheusScript) WithNumaNode(numa string) *PrometheusScript {
	c.NumaNode = numa
	return c
}

// WithRetention set Retention field of PrometheusScript
func (c *PrometheusScript) WithRetention(retention string) *PrometheusScript {
	valid, _ := regexp.MatchString("^[1-9]\\d*d$", retention)
	if retention == "" || !valid {
		c.Retention = "30d"
	} else {
		c.Retention = retention
	}
	return c
}

// WithTPLFile set the template file.
func (c *PrometheusScript) WithTPLFile(fname string) *PrometheusScript {
	c.tplFile = fname
	return c
}

// WithNG set if enable ng-monitoring.
func (c *PrometheusScript) WithNG(ngPort int) *PrometheusScript {
	if ngPort > 0 {
		c.EnableNG = true
	} else {
		c.EnableNG = false
	}
	return c
}

// Config generate the config file data.
func (c *PrometheusScript) Config() ([]byte, error) {
	fp := c.tplFile
	if fp == "" {
		fp = path.Join("templates", "scripts", "run_prometheus.sh.tpl")
	}

	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return nil, err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Prometheus config content by tpl
func (c *PrometheusScript) ConfigWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("Prometheus").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// ConfigToFile write config content to specific path
func (c *PrometheusScript) ConfigToFile(file string) error {
	config, err := c.Config()
	if err != nil {
		return err
	}
	return os.WriteFile(file, config, 0755)
}
