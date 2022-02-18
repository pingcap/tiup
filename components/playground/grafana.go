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

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

type grafana struct {
	host    string
	port    int
	version string

	waitErr  error
	waitOnce sync.Once
	cmd      *exec.Cmd
}

func newGrafana(version string, host string) *grafana {
	return &grafana{
		host:    host,
		version: version,
	}
}

// ref: https://grafana.com/docs/grafana/latest/administration/provisioning/
func writeDatasourceConfig(fname string, clusterName string, p8sURL string) error {
	err := makeSureDir(fname)
	if err != nil {
		return err
	}

	tpl := `apiVersion: 1
datasources:
  - name: %s
    type: prometheus
    access: proxy
    url: %s
    withCredentials: false
    isDefault: false
    tlsAuth: false
    tlsAuthWithCACert: false
    version: 1
    editable: true
`

	s := fmt.Sprintf(tpl, clusterName, p8sURL)
	err = os.WriteFile(fname, []byte(s), 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

// ref: templates/scripts/run_grafana.sh.tpl
// replace the data source in json to the one we are using.
func replaceDatasource(dashboardDir string, datasourceName string) error {
	// for "s/\${DS_.*-CLUSTER}/datasourceName/g
	re := regexp.MustCompile(`\${DS_.*-CLUSTER}`)

	err := filepath.Walk(dashboardDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("skip scan %s failed: %v", path, err)
			return nil
		}

		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return errors.AddStack(err)
		}

		s := string(data)
		s = strings.ReplaceAll(s, "test-cluster", datasourceName)
		s = strings.ReplaceAll(s, "Test-Cluster", datasourceName)
		s = strings.ReplaceAll(s, "${DS_LIGHTNING}", datasourceName)
		s = re.ReplaceAllLiteralString(s, datasourceName)

		return os.WriteFile(path, []byte(s), 0644)
	})

	if err != nil {
		return err
	}

	return nil
}

func writeDashboardConfig(fname string, clusterName string, dir string) error {
	err := makeSureDir(fname)
	if err != nil {
		return err
	}

	tpl := `apiVersion: 1
providers:
  - name: %s
    folder: %s
    type: file
    disableDeletion: false
    editable: true
    updateIntervalSeconds: 30
    options:
      path: %s
`
	s := fmt.Sprintf(tpl, clusterName, clusterName, dir)

	err = os.WriteFile(fname, []byte(s), 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

func makeSureDir(fname string) error {
	return os.MkdirAll(filepath.Dir(fname), 0755)
}

var clusterName string = "playground"

// dir should contains files untar the grafana.
// return not error iff the Cmd is started successfully.
func (g *grafana) start(ctx context.Context, dir string, p8sURL string) (err error) {
	g.port, err = utils.GetFreePort(g.host, 3000)
	if err != nil {
		return err
	}

	fname := filepath.Join(dir, "conf", "provisioning", "dashboards", "dashboard.yml")
	err = writeDashboardConfig(fname, clusterName, filepath.Join(dir, "dashboards"))
	if err != nil {
		return err
	}

	fname = filepath.Join(dir, "conf", "provisioning", "datasources", "datasource.yml")
	err = writeDatasourceConfig(fname, clusterName, p8sURL)
	if err != nil {
		return err
	}

	tpl := `
[server]
# The ip address to bind to, empty will bind to all interfaces
http_addr = %s

# The http port to use
http_port = %d
`
	err = os.MkdirAll(filepath.Join(dir, "conf"), 0755)
	if err != nil {
		return errors.AddStack(err)
	}

	custome := fmt.Sprintf(tpl, g.host, g.port)
	customeFName := filepath.Join(dir, "conf", "custom.ini")

	err = os.WriteFile(customeFName, []byte(custome), 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	args := []string{
		"--homepath",
		dir,
		"--config",
		customeFName,
		fmt.Sprintf("cfg:default.paths.logs=%s", path.Join(dir, "log")),
	}

	var binPath string
	if binPath, err = tiupexec.PrepareBinary("grafana", utils.Version(g.version), binPath); err != nil {
		return err
	}
	cmd := instance.PrepareCommand(ctx, binPath, args, nil, dir)

	g.cmd = cmd

	return g.cmd.Start()
}

func (g *grafana) wait() error {
	g.waitOnce.Do(func() {
		g.waitErr = g.cmd.Wait()
	})

	return g.waitErr
}
