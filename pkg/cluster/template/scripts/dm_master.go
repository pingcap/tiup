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

// DMMasterScript represent the data to generate TiDB config
type DMMasterScript struct {
	Name             string
	V1SourcePath     string
	MasterAddr       string
	AdvertiseAddr    string
	PeerURL          string
	AdvertisePeerURL string
	InitialCluster   string

	DeployDir string
	DataDir   string
	LogDir    string

	NumaNode string
}

// ConfigToFile write config content to specific path
func (c *DMMasterScript) ConfigToFile(file string) error {
	fp := path.Join("templates", "scripts", "run_dm-master.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}
	tmpl, err := template.New("dm-master").Parse(string(tpl))
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

// DMMasterScaleScript represent the data to generate dm-master config on scaling
type DMMasterScaleScript struct {
	Name             string
	V1SourcePath     string
	MasterAddr       string
	AdvertiseAddr    string
	PeerURL          string
	AdvertisePeerURL string
	Join             string

	DeployDir string
	DataDir   string
	LogDir    string

	NumaNode string
}

// ConfigToFile write config content to specific path
func (c *DMMasterScaleScript) ConfigToFile(file string) error {
	fp := path.Join("templates", "scripts", "run_dm-master_scale.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}
	tmpl, err := template.New("dm-master").Parse(string(tpl))
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
