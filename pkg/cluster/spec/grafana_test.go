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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalDashboards(t *testing.T) {
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))

	deployDir, err := os.MkdirTemp("", "tiup-*")
	assert.Nil(t, err)
	defer os.RemoveAll(deployDir)
	localDir, err := filepath.Abs("./testdata/dashboards")
	assert.Nil(t, err)

	topo := new(Specification)
	topo.Grafanas = append(topo.Grafanas, &GrafanaSpec{
		Host:         "127.0.0.1",
		Port:         3000,
		DashboardDir: localDir,
	})

	comp := GrafanaComponent{topo}
	ints := comp.Instances()

	assert.Equal(t, len(ints), 1)
	grafanaInstance := ints[0].(*GrafanaInstance)

	user, err := user.Current()
	assert.Nil(t, err)
	e, err := executor.New(executor.SSHTypeNone, false, executor.SSHConfig{Host: "127.0.0.1", User: user.Username})
	assert.Nil(t, err)

	clusterName := "tiup-test-cluster-" + uuid.New().String()
	err = grafanaInstance.initDashboards(ctx, e, topo.Grafanas[0], meta.DirPaths{Deploy: deployDir}, clusterName)
	assert.Nil(t, err)

	assert.FileExists(t, path.Join(deployDir, "dashboards", "tidb.json"))
	fs, err := os.ReadDir(localDir)
	assert.Nil(t, err)
	for _, f := range fs {
		assert.FileExists(t, path.Join(deployDir, "dashboards", f.Name()))
	}
}

func TestMergeAdditionalGrafanaConf(t *testing.T) {
	file, err := os.CreateTemp("", "tiup-cluster-spec-test")
	if err != nil {
		panic(fmt.Sprintf("create temp file: %s", err))
	}
	defer os.Remove(file.Name())

	_, err = file.WriteString(`#################################### SMTP / Emailing ##########################
[smtp]
;enabled = false
;host = localhost:25
;user =
password = ` + "`1#2`" + `
;cert_file =
;key_file =
;skip_verify = false
;from_address = admin@grafana.localhost

[emails]
;welcome_email_on_sign_up = false

#################################### Logging ##########################
[log]
# Either "console", "file", "syslog". Default is console and  file
# Use space to separate multiple modes, e.g. "console file"
mode = file

# Either "trace", "debug", "info", "warn", "error", "critical", default is "info"
;level = info
# For "console" mode only
[log.console]
;level =

# log line format, valid options are text, console and json
;format = console

# For "file" mode only
[log.file]
level = info
`)
	assert.Nil(t, err)

	expected := `# ################################### SMTP / Emailing ##########################
[smtp]
; enabled = false
; host = localhost:25
; user =
password = ` + "`1#2`" + `
enabled  = true

; cert_file =
; key_file =
; skip_verify = false
; from_address = admin@grafana.localhost
[emails]

; welcome_email_on_sign_up = false
# ################################### Logging ##########################
[log]
# Either "console", "file", "syslog". Default is console and  file
# Use space to separate multiple modes, e.g. "console file"
mode = file

# Either "trace", "debug", "info", "warn", "error", "critical", default is "info"
; level = info
# For "console" mode only
[log.console]

; level =
# log line format, valid options are text, console and json
; format = console
# For "file" mode only
[log.file]
level = warning
`

	addition := map[string]string{
		"log.file.level": "warning",
		"smtp.enabled":   "true",
	}

	err = mergeAdditionalGrafanaConf(file.Name(), addition)
	assert.Nil(t, err)
	result, err := os.ReadFile(file.Name())
	assert.Nil(t, err)

	assert.Equal(t, expected, string(result))
}

type mockExecutor struct {
	executeFunc func(ctx context.Context, cmd string, sudo bool, timeouts ...time.Duration) ([]byte, []byte, error)
}

func (e *mockExecutor) Execute(ctx context.Context, cmd string, sudo bool, timeouts ...time.Duration) (stdout []byte, stderr []byte, err error) {
	if e.executeFunc != nil {
		return e.executeFunc(ctx, cmd, sudo, timeouts...)
	}
	return nil, nil, nil
}

func (e *mockExecutor) Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) error {
	// Copy the file for testing
	if !download {
		err := os.MkdirAll(filepath.Dir(dst), 0755)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, content, 0644)
	}
	return nil
}

func TestGrafanaDatasourceConfig(t *testing.T) {
	ctx := context.Background()
	deployDir := t.TempDir()
	cacheDir := t.TempDir()

	// Create paths structure
	paths := meta.DirPaths{
		Deploy: deployDir,
		Cache:  cacheDir,
	}

	// Create mock executor
	mockExec := &mockExecutor{}

	// Create test topology with both Prometheus and VM
	topo := new(Specification)
	topo.Monitors = []*PrometheusSpec{
		{
			Host:                "127.0.0.1",
			Port:                9090,
			NgPort:              12020,
			PromRemoteWriteToVM: true,
		},
	}
	topo.Grafanas = []*GrafanaSpec{
		{
			Host: "127.0.0.1",
			Port: 3000,
		},
	}

	// Create Grafana component
	comp := GrafanaComponent{topo}
	grafanaInstance := comp.Instances()[0].(*GrafanaInstance)

	// Test datasource configuration
	clusterName := "test-cluster"
	err := grafanaInstance.InitConfig(ctxt.New(ctx, 0, logprinter.NewLogger("")), mockExec, clusterName, "v5.4.0", "tidb", paths, InstanceOpt{})
	require.NoError(t, err)

	// Verify the datasource configuration file
	dsContent, err := os.ReadFile(filepath.Join(deployDir, "provisioning", "datasources", "datasource.yml"))
	require.NoError(t, err)

	// Check if the content contains both Prometheus and VM datasources
	assert.Contains(t, string(dsContent), fmt.Sprintf("name: %s", clusterName))
	assert.Contains(t, string(dsContent), fmt.Sprintf("name: %s-vm", clusterName))
	assert.Contains(t, string(dsContent), "type: prometheus")
	assert.Contains(t, string(dsContent), "url: http://127.0.0.1:9090")

	// Verify Prometheus is the default datasource
	assert.Contains(t, string(dsContent), fmt.Sprintf(`name: %s`, clusterName))
	assert.Contains(t, string(dsContent), `isDefault: true`)
	assert.Contains(t, string(dsContent), `url: http://127.0.0.1:9090`)
	assert.Contains(t, string(dsContent), fmt.Sprintf(`name: %s-vm`, clusterName))
	assert.Contains(t, string(dsContent), `url: http://127.0.0.1:12020`)

	// Test without VM remote write enabled
	topo.Monitors[0].PromRemoteWriteToVM = false
	err = grafanaInstance.InitConfig(ctxt.New(ctx, 0, logprinter.NewLogger("")), mockExec, clusterName, "v5.4.0", "tidb", paths, InstanceOpt{})
	require.NoError(t, err)

	// Verify the datasource configuration file again
	dsContent, err = os.ReadFile(filepath.Join(deployDir, "provisioning", "datasources", "datasource.yml"))
	require.NoError(t, err)

	// Check if the content contains only Prometheus datasource
	assert.Contains(t, string(dsContent), fmt.Sprintf("name: %s", clusterName))
	assert.NotContains(t, string(dsContent), fmt.Sprintf("name: %s-vm", clusterName))
	assert.Contains(t, string(dsContent), "type: prometheus")
	assert.Contains(t, string(dsContent), "url: http://127.0.0.1:9090")
}

// TestVictoriaMetricsDefaultDatasource tests that when Victoria Metrics is set as the default datasource,
// the dashboards correctly use it instead of Prometheus
func TestVictoriaMetricsDefaultDatasource(t *testing.T) {
	ctx := context.Background()
	deployDir := t.TempDir()
	cacheDir := t.TempDir()

	// Create paths structure with folders needed for dashboards
	paths := meta.DirPaths{
		Deploy: deployDir,
		Cache:  cacheDir,
	}

	// Create the necessary directory structure
	dashboardsDir := filepath.Join(deployDir, "dashboards")
	binDir := filepath.Join(deployDir, "bin")
	err := os.MkdirAll(dashboardsDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(binDir, 0755)
	require.NoError(t, err)

	// Create a mock for the execute function to handle the dashboard copy command
	origExecutor := &mockExecutor{
		executeFunc: func(ctx context.Context, cmd string, sudo bool, timeouts ...time.Duration) ([]byte, []byte, error) {
			// Manually perform what the command would do
			if strings.Contains(cmd, "find") && strings.Contains(cmd, "cp") {
				// Create the dashboard file by copying it from bin to dashboards dir
				content, err := os.ReadFile(filepath.Join(binDir, "sample.json"))
				if err != nil {
					return nil, nil, err
				}
				err = os.WriteFile(filepath.Join(dashboardsDir, "sample.json"), content, 0644)
				if err != nil {
					return nil, nil, err
				}
			} else if strings.Contains(cmd, "sed") {
				// Handle the sed command to replace datasource references
				files, err := os.ReadDir(dashboardsDir)
				if err != nil {
					return nil, nil, err
				}

				for _, file := range files {
					if strings.HasSuffix(file.Name(), ".json") {
						content, err := os.ReadFile(filepath.Join(dashboardsDir, file.Name()))
						if err != nil {
							return nil, nil, err
						}

						// Replace datasource references - simulating what sed would do
						modifiedContent := strings.ReplaceAll(string(content),
							`"DS_TEST-CLUSTER"`,
							fmt.Sprintf(`"DS_%s-VM"`, strings.ToUpper("test-cluster")))
						modifiedContent = strings.ReplaceAll(modifiedContent,
							`"text": "test-cluster"`,
							fmt.Sprintf(`"text": "%s-vm"`, "test-cluster"))
						modifiedContent = strings.ReplaceAll(modifiedContent,
							`"value": "test-cluster"`,
							fmt.Sprintf(`"value": "%s-vm"`, "test-cluster"))

						err = os.WriteFile(filepath.Join(dashboardsDir, file.Name()), []byte(modifiedContent), 0644)
						if err != nil {
							return nil, nil, err
						}
					}
				}
			}
			return nil, nil, nil
		},
	}

	// Create a sample dashboard file with datasource references
	dashboardContent := `{
		"annotations": {
			"list": []
		},
		"editable": true,
		"fiscalYearStartMonth": 0,
		"graphTooltip": 0,
		"links": [],
		"liveNow": false,
		"panels": [],
		"refresh": "",
		"schemaVersion": 38,
		"style": "dark",
		"tags": [],
		"templating": {
			"list": [
				{
					"current": {
						"selected": false,
						"text": "test-cluster",
						"value": "test-cluster"
					},
					"hide": 0,
					"includeAll": false,
					"label": "Datasource",
					"multi": false,
					"name": "DS_TEST-CLUSTER",
					"options": [],
					"query": "prometheus",
					"refresh": 1,
					"regex": "",
					"skipUrlSync": false,
					"type": "datasource"
				}
			]
		},
		"title": "Test Dashboard",
		"uid": "test",
		"version": 1,
		"weekStart": ""
	}`
	err = os.WriteFile(filepath.Join(binDir, "sample.json"), []byte(dashboardContent), 0644)
	require.NoError(t, err)

	// Create test topology with VM as default datasource
	topo := new(Specification)
	topo.Monitors = []*PrometheusSpec{
		{
			Host:                "127.0.0.1",
			Port:                9090,
			NgPort:              12020,
			PromRemoteWriteToVM: true,
		},
	}
	topo.Grafanas = []*GrafanaSpec{
		{
			Host:              "127.0.0.1",
			Port:              3000,
			UseVMAsDatasource: true,
		},
	}

	// Create Grafana component with VM as default datasource
	comp := GrafanaComponent{topo}
	grafanaInstance := comp.Instances()[0].(*GrafanaInstance)

	// Run InitConfig which will process dashboards
	err = grafanaInstance.InitConfig(ctxt.New(ctx, 0, logprinter.NewLogger("")), origExecutor, "test-cluster", "v5.4.0", "tidb", paths, InstanceOpt{})
	require.NoError(t, err)

	// Check if the dashboard file was created and datasource references were updated
	dashboardFile := filepath.Join(dashboardsDir, "sample.json")
	content, err := os.ReadFile(dashboardFile)
	require.NoError(t, err)

	// Verify VM datasource was used
	assert.Contains(t, string(content), `"DS_TEST-CLUSTER-VM"`)
	assert.Contains(t, string(content), `"text": "test-cluster-vm"`)
	assert.Contains(t, string(content), `"value": "test-cluster-vm"`)
}
