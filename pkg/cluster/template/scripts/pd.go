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
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/pingcap/tiup/pkg/utils"
)

// PDScript represent the data to generate pd config
type PDScript struct {
	Name               string
	ClientURL          string
	AdvertiseClientURL string
	PeerURL            string
	AdvertisePeerURL   string
	InitialCluster     string

	DeployDir string
	DataDir   string
	LogDir    string

	NumaNode string
	MSMode   bool
}

// ConfigToFile write config content to specific path
func (c *PDScript) ConfigToFile(file string) error {
	fp := path.Join("templates", "scripts", "run_pd.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("PD").Parse(string(tpl))
	if err != nil {
		return err
	}

	if c.Name == "" {
		return errors.New("empty name")
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return err
	}

	return utils.WriteFile(file, content.Bytes(), 0755)
}

// PDScaleScript represent the data to generate pd config on scaling
type PDScaleScript struct {
	PDScript
	Join string
}

// NewPDScaleScript return a new PDScaleScript
func NewPDScaleScript(pdScript *PDScript, join string) *PDScaleScript {
	return &PDScaleScript{PDScript: *pdScript, Join: join}
}

// ConfigToFile write config content to specific path
func (c *PDScaleScript) ConfigToFile(file string) error {
	fp := path.Join("templates", "scripts", "run_pd_scale.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("PD").Parse(string(tpl))
	if err != nil {
		return err
	}

	if c.Name == "" {
		return errors.New("empty name")
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, c); err != nil {
		return err
	}

	return utils.WriteFile(file, content.Bytes(), 0755)
}
