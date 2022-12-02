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

// DashboardScript represents the data structure for generating startup shell scripts,
// with the help of Go Templates.
type DashboardScript struct {
	TidbVersion  string
	IP           string
	Port         int
	DeployDir    string
	LogDir       string
	DataDir      string
	NumaNode     string
	PD           string
	Telemetry    bool
	Experimental bool
	PathPrefix   string

	ClusterTLSEnabled  bool
	ClusterTLSCAPath   string
	ClusterTLSCertPath string
	ClusterTLSKeyPath  string

	MysqlClientTLSEnabled  bool
	MysqlClientTLSCAPath   string
	MysqlClientTLSCertPath string
	MysqlClientTLSKeyPath  string
}

// ConfigToFile write config content to specific file.
func (s *DashboardScript) ConfigToFile(file string) error {
	fp := path.Join("templates", "scripts", "run_tidb-dashboard.sh.tpl")
	tpl, err := embed.ReadTemplate(fp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("tidb-dashboard").Parse(string(tpl))
	if err != nil {
		return err
	}

	content := bytes.NewBufferString("")
	if err := tmpl.Execute(content, s); err != nil {
		return err
	}

	return os.WriteFile(file, content.Bytes(), 0755)
}
