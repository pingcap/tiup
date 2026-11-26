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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
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
	"gopkg.in/ini.v1"
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
	err := grafanaInstance.InitConfig(ctxt.New(ctx, 0, logprinter.NewLogger("")), mockExec, clusterName, "v5.4.0", "tidb", paths)
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
	err = grafanaInstance.InitConfig(ctxt.New(ctx, 0, logprinter.NewLogger("")), mockExec, clusterName, "v5.4.0", "tidb", paths)
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

	datasourceName := "test-cluster-vm"
	dashboardReplacements := []struct {
		commandPattern string
		placeholder    string
	}{
		{`s/test-cluster/`, "test-cluster"},
		{`s/Test-Cluster/`, "Test-Cluster"},
		{`s/\${DS_.*-CLUSTER}/`, "${DS_TEST-CLUSTER}"},
		{`s/DS_.*-CLUSTER/`, "DS_TEST-CLUSTER"},
		{`s/\${DS_LIGHTNING}/`, "${DS_LIGHTNING}"},
		{`s/DS_LIGHTNING/`, "DS_LIGHTNING"},
	}
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
				// Handle each sed command using the same replacement order as initDashboards.
				files, err := os.ReadDir(dashboardsDir)
				if err != nil {
					return nil, nil, err
				}

				for _, file := range files {
					if strings.HasSuffix(file.Name(), ".json") {
						filePath := filepath.Join(dashboardsDir, file.Name())
						content, err := os.ReadFile(filePath)
						if err != nil {
							return nil, nil, err
						}

						modifiedContent := string(content)
						for _, replacement := range dashboardReplacements {
							if strings.Contains(cmd, replacement.commandPattern) {
								modifiedContent = strings.ReplaceAll(modifiedContent, replacement.placeholder, datasourceName)
								break
							}
						}

						err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
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
			"list": [
				{
					"datasource": "${DS_TEST-CLUSTER}"
				}
			]
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
	err = grafanaInstance.InitConfig(ctxt.New(ctx, 0, logprinter.NewLogger("")), origExecutor, "test-cluster", "v5.4.0", "tidb", paths)
	require.NoError(t, err)

	// Check if the dashboard file was created and datasource references were updated
	dashboardFile := filepath.Join(dashboardsDir, "sample.json")
	content, err := os.ReadFile(dashboardFile)
	require.NoError(t, err)

	// Verify every dashboard reference uses the provisioned VM datasource name
	// without rewriting it to test-cluster-vm-vm.
	assert.Contains(t, string(content), `"datasource": "test-cluster-vm"`)
	assert.Contains(t, string(content), `"name": "test-cluster-vm"`)
	assert.Contains(t, string(content), `"text": "test-cluster-vm"`)
	assert.Contains(t, string(content), `"value": "test-cluster-vm"`)
	assert.NotContains(t, string(content), "test-cluster-vm-vm")

	dsContent, err := os.ReadFile(filepath.Join(deployDir, "provisioning", "datasources", "datasource.yml"))
	require.NoError(t, err)
	assert.Contains(t, string(dsContent), "name: test-cluster-vm")
}

// TestGrafanaProbeProtocol checks that the status and uptime probes follow
// server.protocol and do not use the cluster TLS config.
func TestGrafanaProbeProtocol(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			fmt.Fprintf(w, "process_start_time_seconds %d\n", time.Now().Add(-time.Hour).Unix())
		}
	})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	httpsServer := httptest.NewTLSServer(handler)
	t.Cleanup(httpsServer.Close)

	// the cluster TLS config of a cluster with enable_tls, Grafana does not use it
	clusterTLS := &tls.Config{RootCAs: x509.NewCertPool()}

	tests := []struct {
		name           string
		serverConfig   map[string]string
		instanceConfig map[string]string
		tlsServer      bool
		clusterTLS     *tls.Config
		status         string
	}{
		{name: "http by default", status: "Up"},
		{name: "http with cluster tls", clusterTLS: clusterTLS, status: "Up"},
		{name: "https in instance config", instanceConfig: map[string]string{"server.protocol": "https"}, tlsServer: true, status: "Up"},
		{name: "h2 in instance config", instanceConfig: map[string]string{"server.protocol": "h2"}, tlsServer: true, status: "Up"},
		{name: "https in server_configs", serverConfig: map[string]string{"server.protocol": "https"}, tlsServer: true, clusterTLS: clusterTLS, status: "Up"},
		{name: "instance config overrides server_configs", serverConfig: map[string]string{"server.protocol": "https"}, instanceConfig: map[string]string{"server.protocol": "http"}, tlsServer: true, status: "Down"},
		{name: "http probe of https grafana", tlsServer: true, status: "Down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httpServer
			if tt.tlsServer {
				server = httpsServer
			}
			u, err := url.Parse(server.URL)
			require.NoError(t, err)
			port, err := strconv.Atoi(u.Port())
			require.NoError(t, err)

			topo := new(Specification)
			topo.ServerConfigs.Grafana = tt.serverConfig
			topo.Grafanas = append(topo.Grafanas, &GrafanaSpec{
				Host:   u.Hostname(),
				Port:   port,
				Config: tt.instanceConfig,
			})
			ins := (&GrafanaComponent{topo}).Instances()
			require.Len(t, ins, 1)

			ctx := context.Background()
			require.Equal(t, tt.status, ins[0].Status(ctx, 5*time.Second, tt.clusterTLS))
			uptime := ins[0].Uptime(ctx, 5*time.Second, tt.clusterTLS)
			if tt.status == "Up" {
				require.GreaterOrEqual(t, uptime, time.Hour)
			} else {
				require.Zero(t, uptime)
			}
		})
	}
}

// TestGrafanaMergedConfig checks that the instance config overrides
// server_configs.grafana and the shared map is not modified.
func TestGrafanaMergedConfig(t *testing.T) {
	global := map[string]string{"log.level": "info", "server.protocol": "http"}
	a := &GrafanaSpec{Config: map[string]string{"server.protocol": "https"}}
	b := &GrafanaSpec{}

	require.Equal(t, map[string]string{"log.level": "info", "server.protocol": "https"}, a.mergedConfig(global))
	require.Equal(t, map[string]string{"log.level": "info", "server.protocol": "http"}, b.mergedConfig(global))
	require.Equal(t, map[string]string{"log.level": "info", "server.protocol": "http"}, global)
	require.Empty(t, b.mergedConfig(nil))
}

// TestGrafanaInitConfigInstanceOverride checks that the instance config of one
// Grafana does not leak into the grafana.ini of another one.
func TestGrafanaInitConfigInstanceOverride(t *testing.T) {
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))

	topo := new(Specification)
	topo.ServerConfigs.Grafana = map[string]string{"server.enable_gzip": "true"}
	topo.Monitors = []*PrometheusSpec{{Host: "127.0.0.1", Port: 9090}}
	topo.Grafanas = []*GrafanaSpec{
		{Host: "127.0.0.1", Port: 3000, Config: map[string]string{"server.protocol": "https"}},
		{Host: "127.0.0.1", Port: 3001},
	}

	var servers []*ini.Section
	for _, ins := range (&GrafanaComponent{topo}).Instances() {
		paths := meta.DirPaths{Deploy: t.TempDir(), Cache: t.TempDir()}
		require.NoError(t, ins.InitConfig(ctx, &mockExecutor{}, "test-cluster", "v8.5.0", "tidb", paths))
		cfg, err := ini.Load(filepath.Join(paths.Deploy, "conf", "grafana.ini"))
		require.NoError(t, err)
		servers = append(servers, cfg.Section("server"))
	}

	require.Equal(t, "https", servers[0].Key("protocol").String())
	require.Equal(t, "true", servers[0].Key("enable_gzip").String())
	require.False(t, servers[1].HasKey("protocol"))
	require.Equal(t, "true", servers[1].Key("enable_gzip").String())
	require.Equal(t, map[string]string{"server.enable_gzip": "true"}, topo.ServerConfigs.Grafana)
}
