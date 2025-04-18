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

package spec

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestLocalRuleDirs(t *testing.T) {
	deployDir, err := os.MkdirTemp("", "tiup-*")
	assert.Nil(t, err)
	defer os.RemoveAll(deployDir)
	err = utils.MkdirAll(path.Join(deployDir, "bin/prometheus"), 0755)
	assert.Nil(t, err)
	localDir, err := filepath.Abs("./testdata/rules")
	assert.Nil(t, err)

	err = os.WriteFile(path.Join(deployDir, "bin/prometheus", "dummy.rules.yml"), []byte("dummy"), 0644)
	assert.Nil(t, err)

	topo := new(Specification)
	topo.Monitors = append(topo.Monitors, &PrometheusSpec{
		Host:    "127.0.0.1",
		Port:    9090,
		RuleDir: localDir,
	})

	comp := MonitorComponent{topo}
	ints := comp.Instances()

	assert.Equal(t, len(ints), 1)
	promInstance := ints[0].(*MonitorInstance)

	user, err := user.Current()
	assert.Nil(t, err)
	e, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: user.Username})
	assert.Nil(t, err)

	ctx := checkpoint.NewContext(context.Background())
	err = promInstance.initRules(ctx, e, promInstance.InstanceSpec.(*PrometheusSpec), meta.DirPaths{Deploy: deployDir}, "dummy-cluster")
	assert.Nil(t, err)

	assert.FileExists(t, path.Join(deployDir, "conf", "dummy.rules.yml"))
	fs, err := os.ReadDir(localDir)
	assert.Nil(t, err)
	for _, f := range fs {
		assert.FileExists(t, path.Join(deployDir, "conf", f.Name()))
	}
}

func TestNoLocalRuleDirs(t *testing.T) {
	deployDir, err := os.MkdirTemp("", "tiup-*")
	assert.Nil(t, err)
	defer os.RemoveAll(deployDir)
	err = utils.MkdirAll(path.Join(deployDir, "bin/prometheus"), 0755)
	assert.Nil(t, err)
	localDir, err := filepath.Abs("./testdata/rules")
	assert.Nil(t, err)

	err = os.WriteFile(path.Join(deployDir, "bin/prometheus", "dummy.rules.yml"), []byte(`
groups:
  - name: alert.rules
    rules:
      - alert: TiDB_schema_error
        expr: increase(tidb_session_schema_lease_error_total{type="outdated"}[15m]) > 0
        for: 1m
        labels:
          env: ENV_LABELS_ENV
          level: emergency
          expr: increase(tidb_session_schema_lease_error_total{type="outdated"}[15m]) > 0
        annotations:
          description: "cluster: ENV_LABELS_ENV, instance: {{ $labels.instance }}, values:{{ $value }}"
          value: "{{ $value }}"
          summary: TiDB schema error
`), 0644)
	assert.Nil(t, err)

	topo := new(Specification)
	topo.Monitors = append(topo.Monitors, &PrometheusSpec{
		Host: "127.0.0.1",
		Port: 9090,
	})

	comp := MonitorComponent{topo}
	ints := comp.Instances()

	assert.Equal(t, len(ints), 1)
	promInstance := ints[0].(*MonitorInstance)

	user, err := user.Current()
	assert.Nil(t, err)
	e, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: user.Username})
	assert.Nil(t, err)

	ctx := checkpoint.NewContext(context.Background())
	err = promInstance.initRules(ctx, e, promInstance.InstanceSpec.(*PrometheusSpec), meta.DirPaths{Deploy: deployDir}, "dummy-cluster")
	assert.Nil(t, err)
	body, err := os.ReadFile(path.Join(deployDir, "conf", "dummy.rules.yml"))
	assert.Nil(t, err)
	assert.Contains(t, string(body), "dummy-cluster")
	assert.NotContains(t, string(body), "ENV_LABELS_ENV")

	assert.FileExists(t, path.Join(deployDir, "conf", "dummy.rules.yml"))
	fs, err := os.ReadDir(localDir)
	assert.Nil(t, err)
	for _, f := range fs {
		assert.NoFileExists(t, path.Join(deployDir, "conf", f.Name()))
	}
}

func TestMergeAdditionalScrapeConf(t *testing.T) {
	file, err := os.CreateTemp("", "tiup-cluster-spec-test")
	if err != nil {
		panic(fmt.Sprintf("create temp file: %s", err))
	}
	defer os.Remove(file.Name())

	_, err = file.WriteString(`---
global:
  scrape_interval:     15s # By default, scrape targets every 15 seconds.
  evaluation_interval: 15s # By default, scrape targets every 15 seconds.
  # scrape_timeout is set to the global default (10s).
  external_labels:
    cluster: 'test'
    monitor: "prometheus"

scrape_configs:
  - job_name: "tidb"
    honor_labels: true # don't overwrite job & instance labels
    static_configs:
    - targets:
      - '192.168.122.215:10080'
  - job_name: "tikv"
    honor_labels: true # don't overwrite job & instance labels
    static_configs:
    - targets:
      - '192.168.122.25:20180'`)
	assert.Nil(t, err)

	expected := `global:
    evaluation_interval: 15s
    external_labels:
        cluster: test
        monitor: prometheus
    scrape_interval: 15s
scrape_configs:
    - honor_labels: true
      job_name: tidb
      metric_relabel_configs:
        - action: drop
          regex: tikv_thread_nonvoluntary_context_switches|tikv_thread_voluntary_context_switches|tikv_threads_io_bytes_total
          separator: ;
          source_labels:
            - __name__
        - action: drop
          regex: tikv_thread_cpu_seconds_total;(tokio|rocksdb).+
          separator: ;
          source_labels:
            - __name__
            - name
      static_configs:
        - targets:
            - 192.168.122.215:10080
    - honor_labels: true
      job_name: tikv
      metric_relabel_configs:
        - action: drop
          regex: tikv_thread_nonvoluntary_context_switches|tikv_thread_voluntary_context_switches|tikv_threads_io_bytes_total
          separator: ;
          source_labels:
            - __name__
        - action: drop
          regex: tikv_thread_cpu_seconds_total;(tokio|rocksdb).+
          separator: ;
          source_labels:
            - __name__
            - name
      static_configs:
        - targets:
            - 192.168.122.25:20180
`

	var addition map[string]any
	err = yaml.Unmarshal([]byte(`metric_relabel_configs:
  - source_labels: [__name__]
    separator: ;
    regex: tikv_thread_nonvoluntary_context_switches|tikv_thread_voluntary_context_switches|tikv_threads_io_bytes_total
    action: drop
  - source_labels: [__name__,name]
    separator: ;
    regex: tikv_thread_cpu_seconds_total;(tokio|rocksdb).+
    action: drop`), &addition)
	assert.Nil(t, err)

	err = mergeAdditionalScrapeConf(file.Name(), addition)
	assert.Nil(t, err)
	result, err := os.ReadFile(file.Name())
	assert.Nil(t, err)

	assert.Equal(t, expected, string(result))
}

func TestGetRetention(t *testing.T) {
	var val string
	val = getRetention("-1d")
	assert.EqualValues(t, "30d", val)

	val = getRetention("0d")
	assert.EqualValues(t, "30d", val)

	val = getRetention("01d")
	assert.EqualValues(t, "30d", val)

	val = getRetention("1dd")
	assert.EqualValues(t, "30d", val)

	val = getRetention("*1d")
	assert.EqualValues(t, "30d", val)

	val = getRetention("1d ")
	assert.EqualValues(t, "30d", val)

	val = getRetention("ddd")
	assert.EqualValues(t, "30d", val)

	val = getRetention("60d")
	assert.EqualValues(t, "60d", val)

	val = getRetention("999d")
	assert.EqualValues(t, "999d", val)
}

func TestHandleRemoteWrite(t *testing.T) {
	topo := new(Specification)

	// Create monitoring instance
	monitorInstance := &MonitorInstance{
		BaseInstance: BaseInstance{
			InstanceSpec: &PrometheusSpec{},
		},
		topo: topo,
	}

	// Test case 1: VM is disabled
	spec := &PrometheusSpec{
		VMConfig: VMConfig{
			Enable: false,
		},
		RemoteConfig: Remote{},
	}

	monitoring := &PrometheusSpec{
		Host:   "127.0.0.1",
		NgPort: 12020,
	}

	// Call the function
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// Check that no remote write config was added when VM is disabled
	assert.Nil(t, spec.RemoteConfig.RemoteWrite)

	// Test case 2: VM is enabled but NgPort is 0
	spec = &PrometheusSpec{
		VMConfig: VMConfig{
			Enable: true,
		},
		RemoteConfig: Remote{},
	}

	monitoring = &PrometheusSpec{
		Host:   "127.0.0.1",
		NgPort: 0,
	}

	// Call the function
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// Check that no remote write config was added when NgPort is 0
	assert.Nil(t, spec.RemoteConfig.RemoteWrite)

	// Test case 3: VM is enabled and NgPort is set
	spec = &PrometheusSpec{
		VMConfig: VMConfig{
			Enable: true,
		},
		RemoteConfig: Remote{},
	}

	monitoring = &PrometheusSpec{
		Host:   "127.0.0.1",
		NgPort: 12020,
	}

	// Call the function
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// Check that remote write config was added
	assert.NotNil(t, spec.RemoteConfig.RemoteWrite)
	assert.Len(t, spec.RemoteConfig.RemoteWrite, 1)
	assert.Equal(t, "http://127.0.0.1:12020/api/v1/write", spec.RemoteConfig.RemoteWrite[0]["url"])

	// Test case 4: URL already exists in remote write configs
	expectedURL := fmt.Sprintf("http://%s/api/v1/write", "127.0.0.1:12020")
	existingRemoteWrite := []map[string]any{
		{
			"url": expectedURL,
		},
	}

	spec = &PrometheusSpec{
		VMConfig: VMConfig{
			Enable: true,
		},
		RemoteConfig: Remote{
			RemoteWrite: existingRemoteWrite,
		},
	}

	monitoring = &PrometheusSpec{
		Host:   "127.0.0.1",
		NgPort: 12020,
	}

	// Call the function
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// Check that no duplicate remote write config was added
	assert.Len(t, spec.RemoteConfig.RemoteWrite, 1)
	assert.Equal(t, expectedURL, spec.RemoteConfig.RemoteWrite[0]["url"])
}

// TestVMConfigField tests that the VMConfig field is properly defined in PrometheusSpec
func TestVMConfigField(t *testing.T) {
	// Create a PrometheusSpec with VMConfig
	spec := PrometheusSpec{
		Host: "127.0.0.1",
		Port: 9090,
		VMConfig: VMConfig{
			Enable:              true,
			IsDefaultDatasource: true,
		},
	}

	// Validate VMConfig field exists and is accessible
	assert.True(t, spec.VMConfig.Enable)
	assert.True(t, spec.VMConfig.IsDefaultDatasource)

	// Test setting fields
	spec.VMConfig.Enable = false
	spec.VMConfig.IsDefaultDatasource = false

	assert.False(t, spec.VMConfig.Enable)
	assert.False(t, spec.VMConfig.IsDefaultDatasource)
}

// TestVMConfigYAMLBackwardsCompatibility tests loading YAML with and without VMConfig field
func TestVMConfigYAMLBackwardsCompatibility(t *testing.T) {
	// Old YAML without VMConfig
	oldYAML := `
host: 127.0.0.1
port: 9090
ng_port: 12020
`

	// New YAML with VMConfig
	newYAML := `
host: 127.0.0.1
port: 9090
ng_port: 12020
vm_config:
  enable: true
  is_default_datasource: true
`

	// Test unmarshaling old YAML
	var oldSpec PrometheusSpec
	err := yaml.Unmarshal([]byte(oldYAML), &oldSpec)
	assert.NoError(t, err)

	// Default values should be false
	assert.False(t, oldSpec.VMConfig.Enable)
	assert.False(t, oldSpec.VMConfig.IsDefaultDatasource)

	// Test unmarshaling new YAML
	var newSpec PrometheusSpec
	err = yaml.Unmarshal([]byte(newYAML), &newSpec)
	assert.NoError(t, err)

	// New values should match what's in the YAML
	assert.True(t, newSpec.VMConfig.Enable)
	assert.True(t, newSpec.VMConfig.IsDefaultDatasource)
}
