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

package ansible

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/dm/spec"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/require"
)

// localExecutor only used for test.
type localExecutor struct {
	host string
	executor.Local
}

type executorGetter struct {
}

var _ ExecutorGetter = &executorGetter{}

// Get implements ExecutorGetter interface.
func (g *executorGetter) Get(host string) ctxt.Executor {
	return &localExecutor{
		host: host,
	}
}

// Transfer implements executor interface.
// Replace the deploy directory as the local one in testdata, so we can fetch it.
func (l *localExecutor) Transfer(ctx context.Context, src, target string, download bool, limit int, _ bool) error {
	mydeploy, err := filepath.Abs("./testdata/deploy_dir/" + l.host)
	if err != nil {
		return errors.AddStack(err)
	}
	src = strings.Replace(src, "/home/tidb/deploy", mydeploy, 1)
	return l.Local.Transfer(ctx, src, target, download, 0, false)
}

func TestParseRunScript(t *testing.T) {
	assert := require.New(t)

	// parse run_dm-master.sh
	data, err := os.ReadFile("./testdata/deploy_dir/172.19.0.101/scripts/run_dm-master.sh")
	assert.Nil(err)
	dir, flags, err := parseRunScript(data)
	assert.Nil(err)
	assert.Equal("/home/tidb/deploy", dir)
	expectedFlags := map[string]string{
		"master-addr": ":8261",
		"L":           "info",
		"config":      "conf/dm-master.toml",
		"log-file":    "/home/tidb/deploy/log/dm-master.log",
	}
	assert.Equal(expectedFlags, flags)

	// parse run_dm-worker.sh
	data, err = os.ReadFile("./testdata/deploy_dir/172.19.0.101/scripts/run_dm-worker.sh")
	assert.Nil(err)
	dir, flags, err = parseRunScript(data)
	assert.Nil(err)
	assert.Equal("/home/tidb/deploy", dir)
	expectedFlags = map[string]string{
		"worker-addr": ":8262",
		"L":           "info",
		"relay-dir":   "/home/tidb/deploy/relay_log",
		"config":      "conf/dm-worker.toml",
		"log-file":    "/home/tidb/deploy/log/dm-worker.log",
	}
	assert.Equal(expectedFlags, flags)

	// parse run_prometheus.sh
	data, err = os.ReadFile("./testdata/deploy_dir/172.19.0.101/scripts/run_prometheus.sh")
	assert.Nil(err)
	dir, flags, err = parseRunScript(data)
	assert.Nil(err)
	assert.Equal("/home/tidb/deploy", dir)
	expectedFlags = map[string]string{
		"STDOUT":                 "/home/tidb/deploy/log/prometheus.log",
		"config.file":            "/home/tidb/deploy/conf/prometheus.yml",
		"web.listen-address":     ":9090",
		"web.external-url":       "http://172.19.0.101:9090/",
		"log.level":              "info",
		"storage.tsdb.path":      "/home/tidb/deploy/prometheus.data.metrics",
		"storage.tsdb.retention": "15d",
	}
	assert.Equal(expectedFlags, flags)

	// parse run_grafana.sh
	data, err = os.ReadFile("./testdata/deploy_dir/172.19.0.101/scripts/run_grafana.sh")
	assert.Nil(err)
	dir, flags, err = parseRunScript(data)
	assert.Nil(err)
	assert.Equal("/home/tidb/deploy", dir)
	expectedFlags = map[string]string{
		"homepath": "/home/tidb/deploy/opt/grafana",
		"config":   "/home/tidb/deploy/opt/grafana/conf/grafana.ini",
	}
	assert.Equal(expectedFlags, flags)

	// parse run_alertmanager.sh
	data, err = os.ReadFile("./testdata/deploy_dir/172.19.0.101/scripts/run_alertmanager.sh")
	assert.Nil(err)
	dir, flags, err = parseRunScript(data)
	assert.Nil(err)
	assert.Equal("/home/tidb/deploy", dir)
	expectedFlags = map[string]string{
		"STDOUT":             "/home/tidb/deploy/log/alertmanager.log",
		"config.file":        "conf/alertmanager.yml",
		"storage.path":       "/home/tidb/deploy/data.alertmanager",
		"data.retention":     "120h",
		"log.level":          "info",
		"web.listen-address": ":9093",
	}
	assert.Equal(expectedFlags, flags)
}

func TestImportFromAnsible(t *testing.T) {
	assert := require.New(t)
	dir := "./testdata/ansible"

	im, err := NewImporter(dir, "inventory.ini", executor.SSHTypeBuiltin, 0)
	assert.Nil(err)
	im.testExecutorGetter = &executorGetter{}
	clusterName, meta, err := im.ImportFromAnsibleDir(ctxt.New(
		context.Background(),
		0,
		logprinter.NewLogger(""),
	))
	assert.Nil(err, "verbose: %+v", err)
	assert.Equal("test-cluster", clusterName)

	assert.Equal("tidb", meta.User)
	assert.Equal("v1.0.6", meta.Version)

	// check GlobalOptions
	topo := meta.Topology
	assert.Equal("/home/tidb/deploy", topo.GlobalOptions.DeployDir)
	assert.Equal("/home/tidb/deploy/log", topo.GlobalOptions.LogDir)

	// check master
	assert.Len(topo.Masters, 1)
	master := topo.Masters[0]
	expectedMaster := &spec.MasterSpec{
		Host:      "172.19.0.101",
		SSHPort:   22,
		Port:      8261,
		DeployDir: "",
		LogDir:    "/home/tidb/deploy/log",
		Config:    map[string]any{"log-level": "info"},
		Imported:  true,
	}
	assert.Equal(expectedMaster, master)

	// check worker
	assert.Len(topo.Workers, 2)
	if topo.Workers[0].Host > topo.Workers[1].Host {
		topo.Workers[0], topo.Workers[1] = topo.Workers[1], topo.Workers[0]
	}

	expectedWorker := &spec.WorkerSpec{
		Host:      "172.19.0.101",
		SSHPort:   22,
		Port:      8262,
		DeployDir: "/home/tidb/deploy",
		LogDir:    "/home/tidb/deploy/log",
		Config:    map[string]any{"log-level": "info"},
		Imported:  true,
	}

	worker := topo.Workers[0]
	assert.Equal(expectedWorker, worker)

	expectedWorker.Host = "172.19.0.102"
	worker = topo.Workers[1]
	assert.Equal(expectedWorker, worker)

	// check Alertmanager
	assert.Len(topo.Alertmanagers, 1)
	aler := topo.Alertmanagers[0]
	expectedAlter := &spec.AlertmanagerSpec{
		Host:      "172.19.0.101",
		SSHPort:   22,
		WebPort:   9093,
		DeployDir: "",
		DataDir:   "/home/tidb/deploy/data.alertmanager",
		LogDir:    "/home/tidb/deploy/log",
		Imported:  true,
	}
	assert.Equal(expectedAlter, aler)

	// Check Grafana
	assert.Len(topo.Grafanas, 1)
	grafana := topo.Grafanas[0]
	expectedGrafana := &spec.GrafanaSpec{
		Host:      "172.19.0.101",
		SSHPort:   22,
		DeployDir: "",
		Port:      3001,
		Username:  "foo",
		Password:  "bar",
		Imported:  true,
	}
	assert.Equal(expectedGrafana, grafana)

	// Check Monitor(Prometheus)
	assert.Len(topo.Monitors, 1)
	monitor := topo.Monitors[0]
	expectedMonitor := &spec.PrometheusSpec{
		Host:      "172.19.0.101",
		SSHPort:   22,
		DeployDir: "",
		DataDir:   "/home/tidb/deploy/prometheus.data.metrics",
		LogDir:    "/home/tidb/deploy/log",
		Port:      9090,
		Imported:  true,
	}
	assert.Equal(expectedMonitor, monitor)

	// Check sources
	assert.Len(im.sources, 2)
	s := im.sources[topo.Workers[0].Host+":"+strconv.Itoa(topo.Workers[0].Port)]
	assert.Equal("mysql-replica-01", s.SourceID)
	assert.Equal(DBConfig{
		Host:     "mysql1",
		Password: "password1",
		Port:     3306,
		User:     "root",
	}, s.From)

	s = im.sources[topo.Workers[1].Host+":"+strconv.Itoa(topo.Workers[1].Port)]
	assert.Equal("mysql-replica-02", s.SourceID)
	assert.Equal(DBConfig{
		Host: "mysql2",
		Port: 3306,
		User: "root",
	}, s.From)
}
