// Copyright 2024 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package scripts

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScheduling(t *testing.T) {
	assert := require.New(t)
	conf, err := os.CreateTemp("", "scheduling.conf")
	assert.Nil(err)
	defer os.Remove(conf.Name())

	cfg := &SchedulingScript{
		Name:               "scheduling-0",
		ListenURL:          "127.0.0.1",
		AdvertiseListenURL: "127.0.0.2",
		BackendEndpoints:   "127.0.0.3",
		DeployDir:          "/deploy",
		DataDir:            "/data",
		LogDir:             "/log",
	}
	err = cfg.ConfigToFile(conf.Name())
	assert.Nil(err)
	content, err := os.ReadFile(conf.Name())
	assert.Nil(err)
	assert.True(strings.Contains(string(content), "--name"))

	cfg.Name = ""
	err = cfg.ConfigToFile(conf.Name())
	assert.Nil(err)
	content, err = os.ReadFile(conf.Name())
	assert.Nil(err)
	assert.False(strings.Contains(string(content), "--name"))
}

func TestTSO(t *testing.T) {
	assert := require.New(t)
	conf, err := os.CreateTemp("", "tso.conf")
	assert.Nil(err)
	defer os.Remove(conf.Name())

	cfg := &TSOScript{
		Name:               "tso-0",
		ListenURL:          "127.0.0.1",
		AdvertiseListenURL: "127.0.0.2",
		BackendEndpoints:   "127.0.0.3",
		DeployDir:          "/deploy",
		DataDir:            "/data",
		LogDir:             "/log",
	}
	err = cfg.ConfigToFile(conf.Name())
	assert.Nil(err)
	content, err := os.ReadFile(conf.Name())
	assert.Nil(err)
	assert.True(strings.Contains(string(content), "--name"))

	cfg.Name = ""
	err = cfg.ConfigToFile(conf.Name())
	assert.Nil(err)
	content, err = os.ReadFile(conf.Name())
	assert.Nil(err)
	assert.False(strings.Contains(string(content), "--name"))
}
