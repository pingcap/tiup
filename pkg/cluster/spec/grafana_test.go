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

	"github.com/google/uuid"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/stretchr/testify/assert"
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
;password =
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
enabled = true

; enabled = false
; host = localhost:25
; user =
; password =
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
