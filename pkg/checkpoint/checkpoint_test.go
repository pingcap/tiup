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

package checkpoint

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	sshCmd FieldSet
	scpCmd FieldSet
)

func setup() {
	DebugCheckpoint = true

	// register checkpoint for ssh command
	sshCmd = Register(
		Field("host", reflect.DeepEqual),
		Field("port", func(a, b any) bool {
			return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
		}),
		Field("user", reflect.DeepEqual),
		Field("sudo", reflect.DeepEqual),
		Field("cmd", reflect.DeepEqual),
	)

	// register checkpoint for scp command
	scpCmd = Register(
		Field("host", reflect.DeepEqual),
		Field("port", func(a, b any) bool {
			return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
		}),
		Field("user", reflect.DeepEqual),
		Field("src", reflect.DeepEqual),
		Field("dst", reflect.DeepEqual),
		Field("download", reflect.DeepEqual),
	)
}

func TestCheckPointSimple(t *testing.T) {
	setup()

	assert := require.New(t)
	r := strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "sudo": false, "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": "", "__func__": "test", "__hash__": "Unknown"}`)

	c, err := NewCheckPoint(r)
	assert.Nil(err)
	ctx := NewContext(context.Background())

	p := c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
		"sudo": false,
	})
	assert.NotNil(p.Hit())
	assert.Equal(p.Hit()["stdout"], "success")
	assert.True(p.acquired)

	p1 := c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.139",
	})
	assert.Nil(p1.Hit())
	assert.False(p1.acquired)
	p1.Release(nil)

	p2 := c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.138",
	})
	assert.Nil(p2.Hit())
	assert.False(p2.acquired)
	p1.Release(nil)

	p.Release(nil)

	p3 := c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.137",
	})
	assert.Nil(p3.Hit())
	assert.True(p3.acquired)
	p3.Release(nil)
}

func TestCheckPointMultiple(t *testing.T) {
	setup()

	assert := require.New(t)
	r := strings.NewReader(`
		2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test  cmd", "sudo": false, "stdout": "success", "stderr": "", "__func__": "test", "__hash__": "Unknown"}
		2021-01-14T12:17:32.222+0800    DEBUG   Environment variables {"env": ["TIUP_HOME=/home/tidb/.tiup"}
		2021-01-14T12:17:33.579+0800    INFO    Execute command {"command": "tiup cluster deploy test v4.0.9 /Users/joshua/test.yaml"}
		2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.141", "port": "22", "user": "tidb", "src": "src", "dst": "dst", "download": false, "__func__": "test", "__hash__": "Unknown"}
	`)

	c, err := NewCheckPoint(r)
	assert.Nil(err)
	ctx := NewContext(context.Background())

	p := c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"sudo": false,
		"cmd":  "test  cmd",
	})
	assert.NotNil(p.Hit())
	assert.Equal(p.Hit()["stdout"], "success")
	p.Release(nil)

	p = c.acquire(ctx, scpCmd, "test", map[string]any{
		"host":     "172.16.5.141",
		"port":     22,
		"user":     "tidb",
		"src":      "src",
		"dst":      "dst",
		"download": false,
	})
	assert.NotNil(p.Hit())
	p.Release(nil)
}

func TestCheckPointNil(t *testing.T) {
	setup()

	assert := require.New(t)
	// With wrong log level
	r := strings.NewReader(`2021-01-14T12:16:54.579+0800    ERROR    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err := NewCheckPoint(r)
	assert.Nil(err)
	ctx := NewContext(context.Background())

	p := c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p.Hit())
	p.Release(nil)

	// With wrong log title
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    XXXCommand      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.acquire(ctx, scpCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p.Hit())
	p.Release(nil)

	// With wrong log host
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.141", "port": "22", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p.Hit())
	p.Release(nil)

	// With wrong port
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "23", "user": "tidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p.Hit())
	p.Release(nil)

	// With wrong user
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "yidb", "cmd": "test  cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p.Hit())
	p.Release(nil)

	// With wrong cmd
	r = strings.NewReader(`2021-01-14T12:16:54.579+0800    INFO    CheckPoint      {"host": "172.16.5.140", "port": "22", "user": "tidb", "cmd": "test cmd", "stdout": "success", "stderr": ""}`)

	c, err = NewCheckPoint(r)
	assert.Nil(err)

	p = c.acquire(ctx, sshCmd, "test", map[string]any{
		"host": "172.16.5.140",
		"port": 22,
		"user": "tidb",
		"cmd":  "test  cmd",
	})
	assert.Nil(p.Hit())
	assert.True(p.acquired)
	p.Release(nil)
}

func TestCheckPointNotInited(t *testing.T) {
	setup()

	assert := require.New(t)
	assert.Panics(func() { Acquire(context.Background(), sshCmd, map[string]any{}) })
}
