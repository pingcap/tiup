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
	"text/template"

	"github.com/pingcap/tiup/pkg/utils"
)

// TiSparkEnv represent the data to generate TiSpark environment config
type TiSparkEnv struct {
	Host           string
	TiSparkMaster  string
	TiSparkLocalIP string
	MasterPort     int
	WorkerPort     int
	MasterUIPort   int
	WorkerUIPort   int
	CustomEnvs     map[string]string
}

// NewTiSparkEnv returns a TiSparkConfig
func NewTiSparkEnv(host string) *TiSparkEnv {
	return &TiSparkEnv{Host: host}
}

// WithMaster sets master field
func (c *TiSparkEnv) WithMaster(master string) *TiSparkEnv {
	c.TiSparkMaster = master
	return c
}

// WithCustomEnv sets custom setting fields
func (c *TiSparkEnv) WithCustomEnv(m map[string]string) *TiSparkEnv {
	c.CustomEnvs = m
	return c
}

// WithLocalIP sets custom setting fields
func (c *TiSparkEnv) WithLocalIP(ip string) *TiSparkEnv {
	c.TiSparkLocalIP = ip
	if ip == "0.0.0.0" {
		c.TiSparkLocalIP = "" // use empty result to fall back to spark's default
	}
	return c
}

// WithMasterPorts sets port for masters
func (c *TiSparkEnv) WithMasterPorts(port, ui int) *TiSparkEnv {
	c.MasterPort = port
	c.MasterUIPort = ui
	return c
}

// WithWorkerPorts sets port for masters
func (c *TiSparkEnv) WithWorkerPorts(port, ui int) *TiSparkEnv {
	c.WorkerPort = port
	c.WorkerUIPort = ui
	return c
}

// Script generate the script file data.
func (c *TiSparkEnv) Script() ([]byte, error) {
	tpl, err := GetScript("spark-env.sh.tpl")
	if err != nil {
		return nil, err
	}
	return c.ScriptWithTemplate(string(tpl))
}

// ScriptToFile write script content to specific path
func (c *TiSparkEnv) ScriptToFile(file string) error {
	script, err := c.Script()
	if err != nil {
		return err
	}
	return utils.WriteFile(file, script, 0755)
}

// ScriptWithTemplate parses the template file
func (c *TiSparkEnv) ScriptWithTemplate(tpl string) ([]byte, error) {
	tmpl, err := template.New("TiSpark").Parse(tpl)
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// SlaveScriptWithTemplate parses the template file
func (c *TiSparkEnv) SlaveScriptWithTemplate() ([]byte, error) {
	tpl, err := GetScript("start_tispark_slave.sh.tpl")
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("TiSpark").Parse(string(tpl))
	if err != nil {
		return nil, err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}
