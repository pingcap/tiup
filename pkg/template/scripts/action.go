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

// ActionScript represent the data to generate Action config
type ActionScript struct {
	Action  string
	Service string
}

// NewActionScript returns a ActionScript with given arguments
func NewActionScript(action, service string) *ActionScript {
	return &ActionScript{
		Action:  action,
		Service: service,
	}
}

// Config read ${localdata.EnvNameComponentInstallDir}/templates/scripts/action.sh.tpl as template
// and generate the config by ConfigWithTemplate
func (c *ActionScript) Config() (string, error) {
	fp := path.Join(os.Getenv(localdata.EnvNameComponentInstallDir), "templates", "scripts", "action.sh.tpl")
	tpl, err := ioutil.ReadFile(fp)
	if err != nil {
		return "", err
	}
	return c.ConfigWithTemplate(string(tpl))
}

// ConfigWithTemplate generate the Action config content by tpl
func (c *ActionScript) ConfigWithTemplate(tpl string) (string, error) {
	tmpl, err := template.New("action").Parse(tpl)
	if err != nil {
		return "", err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return "", err
	}

	return content.String(), nil
}
