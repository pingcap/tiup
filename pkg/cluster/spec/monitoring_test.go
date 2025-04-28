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

// TestHandleRemoteWrite verifies that remote write configurations are properly handled
func TestHandleRemoteWrite(t *testing.T) {
	// Create spec and monitoring instances
	spec := &PrometheusSpec{
		Host:                "192.168.1.10",
		Port:                9090,
		PromRemoteWriteToVM: true,
	}
	monitoring := &PrometheusSpec{
		Host:   "192.168.1.20",
		NgPort: 12020,
	}

	// Set up expected remote write URL
	expectedURL := fmt.Sprintf("http://%s/api/v1/write", utils.JoinHostPort(monitoring.Host, monitoring.NgPort))

	monitorInstance := &MonitorInstance{
		BaseInstance: BaseInstance{
			InstanceSpec: spec,
			Host:         spec.Host,
			Port:         spec.Port,
			SSHP:         22,
		},
	}

	// Execute handleRemoteWrite
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// Check remote write config was added
	assert.Len(t, spec.RemoteConfig.RemoteWrite, 1)
	assert.Equal(t, expectedURL, spec.RemoteConfig.RemoteWrite[0]["url"])

	// Add the same remote write URL again
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// Check that no duplicate remote write config was added
	assert.Len(t, spec.RemoteConfig.RemoteWrite, 1)
	assert.Equal(t, expectedURL, spec.RemoteConfig.RemoteWrite[0]["url"])
}

// TestPromRemoteWriteToVM tests remote write configuration
func TestPromRemoteWriteToVM(t *testing.T) {
	// Create a PrometheusSpec with PromRemoteWriteToVM
	spec := PrometheusSpec{
		Host:                "127.0.0.1",
		Port:                9090,
		PromRemoteWriteToVM: true,
	}

	// Validate field is accessible
	assert.True(t, spec.PromRemoteWriteToVM)

	// Test setting field
	spec.PromRemoteWriteToVM = false
	assert.False(t, spec.PromRemoteWriteToVM)
}

// TestVMRemoteWriteYAMLBackwardsCompatibility tests loading YAML with and without PromRemoteWriteToVM field
func TestVMRemoteWriteYAMLBackwardsCompatibility(t *testing.T) {
	// Old YAML without PromRemoteWriteToVM
	oldYAML := `
host: 127.0.0.1
port: 9090
ng_port: 12020
`

	// New YAML with PromRemoteWriteToVM
	newYAML := `
host: 127.0.0.1
port: 9090
ng_port: 12020
prom_remote_write_to_vm: true
`

	// Test unmarshaling old YAML
	var oldSpec PrometheusSpec
	err := yaml.Unmarshal([]byte(oldYAML), &oldSpec)
	assert.NoError(t, err)

	// Default value should be false
	assert.False(t, oldSpec.PromRemoteWriteToVM)

	// Test unmarshaling new YAML
	var newSpec PrometheusSpec
	err = yaml.Unmarshal([]byte(newYAML), &newSpec)
	assert.NoError(t, err)

	// New value should match what's in the YAML
	assert.True(t, newSpec.PromRemoteWriteToVM)
}

// TestHandleRemoteWriteDisabled tests that VM remote write configuration is removed when PromRemoteWriteToVM is false
func TestHandleRemoteWriteDisabled(t *testing.T) {
	// Create spec with existing remote write config and PromRemoteWriteToVM=false
	spec := &PrometheusSpec{
		Host:                "192.168.1.10",
		Port:                9090,
		PromRemoteWriteToVM: false,
	}
	monitoring := &PrometheusSpec{
		Host:   "192.168.1.20",
		NgPort: 12020,
	}

	// Add VM remote write URL
	vmURL := fmt.Sprintf("http://%s/api/v1/write", utils.JoinHostPort(monitoring.Host, monitoring.NgPort))
	spec.RemoteConfig.RemoteWrite = []map[string]any{
		{"url": vmURL},
	}

	// Add another remote write URL that should be preserved
	otherURL := "http://some-other-target:9090/api/v1/write"
	spec.RemoteConfig.RemoteWrite = append(spec.RemoteConfig.RemoteWrite, map[string]any{
		"url": otherURL,
	})

	monitorInstance := &MonitorInstance{
		BaseInstance: BaseInstance{
			InstanceSpec: spec,
			Host:         spec.Host,
			Port:         spec.Port,
			SSHP:         22,
		},
	}

	// Execute handleRemoteWrite with PromRemoteWriteToVM=false
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// Check that VM remote write config was removed but other config was preserved
	assert.Len(t, spec.RemoteConfig.RemoteWrite, 1)
	assert.Equal(t, otherURL, spec.RemoteConfig.RemoteWrite[0]["url"])

	// Test with no remote write configs
	spec.RemoteConfig.RemoteWrite = nil
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// No remote write configs should still be nil/empty
	assert.Empty(t, spec.RemoteConfig.RemoteWrite)

	// Now test with PromRemoteWriteToVM toggled back to true
	spec.PromRemoteWriteToVM = true
	monitorInstance.handleRemoteWrite(spec, monitoring)

	// VM remote write config should be added back
	assert.Len(t, spec.RemoteConfig.RemoteWrite, 1)
	assert.Equal(t, vmURL, spec.RemoteConfig.RemoteWrite[0]["url"])
}
