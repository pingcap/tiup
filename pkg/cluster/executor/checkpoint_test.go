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

package executor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckPointSimple(t *testing.T) {
	assert := require.New(t)
	r := strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err := NewCheckPoint(r)
	assert.Nil(err)

	p := c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.NotNil(p)
	assert.Equal(p["stdout"], "success")
}

func TestCheckPointMultiple(t *testing.T) {
	assert := require.New(t)
	r := strings.NewReader(`
		2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test  cmd", "sudo": false, "stdout": "success", "stderr": ""}
		2021-01-14T12:17:32.222+0800    DEBUG   Environment variables {"env": ["TIUP_HOME=/home/tidb/.tiup"}
		2021-01-14T12:17:33.579+0800    INFO    Execute command {"command": "tiup cluster deploy test v4.0.9 /Users/joshua/test.yaml"}
		2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.141", "port": "22", "user": "tidb", "src": "src", "dst": "dst", "download": false}
	`)

	c, err := NewCheckPoint(r)
	assert.Nil(err)

	p := c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"sudo": false,
		"cmd":  "test  cmd",
	})
	assert.NotNil(p)
	assert.Equal(p["stdout"], "success")

	p = c.Check(map[string]interface{}{
		"host":     "172.16.5.141",
		"port":     22,
		"user":     "tidb",
		"src":      "src",
		"dst":      "dst",
		"download": false,
	})
	assert.NotNil(p)
}

func TestCheckPointNil(t *testing.T) {
	assert := require.New(t)
	// With wrong log level
	r := strings.NewReader(`2021-01-14T12:16:54.579+0800    ERROR    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err := NewCheckPoint(r)
	assert.Nil(err)

	p := c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p)

	// With wrong log title
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    XXXCommand      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p)

	// With wrong log host
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.141", "port": "22", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p)

	// With wrong port
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "23", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p)

	// With wrong user
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "yidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p)

	// With wrong cmd
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.Check(map[string]interface{}{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p)
}
