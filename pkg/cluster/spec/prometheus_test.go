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
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/stretchr/testify/assert"
)

func TestLocalRuleDirs(t *testing.T) {
	deployDir, err := ioutil.TempDir("", "tiup-*")
	assert.Nil(t, err)
	defer os.RemoveAll(deployDir)
	err = os.MkdirAll(path.Join(deployDir, "bin/prometheus"), 0755)
	assert.Nil(t, err)
	localDir, err := filepath.Abs("./testdata/rules")
	assert.Nil(t, err)

	err = ioutil.WriteFile(path.Join(deployDir, "bin/prometheus", "dummy.rules.yml"), []byte("dummy"), 0644)
	assert.Nil(t, err)

	topo := new(Specification)
	topo.Monitors = append(topo.Monitors, PrometheusSpec{
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
	err = promInstance.initRules(ctx, e, promInstance.InstanceSpec.(PrometheusSpec), meta.DirPaths{Deploy: deployDir})
	assert.Nil(t, err)

	assert.NoFileExists(t, path.Join(deployDir, "conf", "dummy.rules.yml"))
	fs, err := ioutil.ReadDir(localDir)
	assert.Nil(t, err)
	for _, f := range fs {
		assert.FileExists(t, path.Join(deployDir, "conf", f.Name()))
	}
}

func TestNoLocalRuleDirs(t *testing.T) {
	deployDir, err := ioutil.TempDir("", "tiup-*")
	assert.Nil(t, err)
	defer os.RemoveAll(deployDir)
	err = os.MkdirAll(path.Join(deployDir, "bin/prometheus"), 0755)
	assert.Nil(t, err)
	localDir, err := filepath.Abs("./testdata/rules")
	assert.Nil(t, err)

	err = ioutil.WriteFile(path.Join(deployDir, "bin/prometheus", "dummy.rules.yml"), []byte("dummy"), 0644)
	assert.Nil(t, err)

	topo := new(Specification)
	topo.Monitors = append(topo.Monitors, PrometheusSpec{
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
	err = promInstance.initRules(ctx, e, promInstance.InstanceSpec.(PrometheusSpec), meta.DirPaths{Deploy: deployDir})
	assert.Nil(t, err)

	assert.FileExists(t, path.Join(deployDir, "conf", "dummy.rules.yml"))
	fs, err := ioutil.ReadDir(localDir)
	assert.Nil(t, err)
	for _, f := range fs {
		assert.NoFileExists(t, path.Join(deployDir, "conf", f.Name()))
	}
}
