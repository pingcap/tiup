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
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

// PrometheusScript represent the data to generate Prometheus config
type PrometheusScript struct {
	Port              int
	WebExternalURL    string
	BasicAuthPassword string
	Retention         string
	EnableNG          bool

	DeployDir string
	DataDir   string
	LogDir    string

	NumaNode string

	AdditionalArgs []string
}

// ConfigToFile write config content to specific path
func (c *PrometheusScript) ConfigToFile(file string) error {
	fp := path.Join("templates", "scripts", "run_prometheus.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("Prometheus").Parse(string(tpl))
	if err != nil {
		return err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return err
	}

	return utils.WriteFile(file, content.Bytes(), 0755)
}

func (c *PrometheusScript) WebConfigToFile(file string) error {
	fp := path.Join("templates", "config", "web_config.yml.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}
	tmpl, err := template.New("web_config").Parse(string(tpl))
	if err != nil {
		return err
	}

	pswHash, err := bcrypt.GenerateFromPassword([]byte(c.BasicAuthPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tmp := struct {
		BasicAuthPassword string
	}{BasicAuthPassword: string(pswHash)}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, tmp); err != nil {
		return err
	}

	return os.WriteFile(file, content.Bytes(), 0755)
}
