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
	"context"
	"fmt"
	"os"
	"os/user"
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/require"
)

func TestLocal(t *testing.T) {
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))

	assert := require.New(t)
	user, err := user.Current()
	assert.Nil(err)
	local, err := New(SSHTypeNone, false, SSHConfig{Host: "127.0.0.1", User: user.Username})
	assert.Nil(err)
	_, _, err = local.Execute(ctx, "ls .", false)
	assert.Nil(err)

	// generate a src file and write some data
	src, err := os.CreateTemp("", "")
	assert.Nil(err)
	defer os.Remove(src.Name())

	n, err := src.WriteString("src")
	assert.Nil(err)
	assert.Equal(3, n)
	err = src.Close()
	assert.Nil(err)

	// generate a dst file and just close it.
	dst, err := os.CreateTemp("", "")
	assert.Nil(err)
	err = dst.Close()
	assert.Nil(err)
	defer os.Remove(dst.Name())

	// Transfer src to dst and check it.
	err = local.Transfer(ctx, src.Name(), dst.Name(), false, 0, false)
	assert.Nil(err)

	data, err := os.ReadFile(dst.Name())
	assert.Nil(err)
	assert.Equal("src", string(data))
}

func TestWrongIP(t *testing.T) {
	assert := require.New(t)
	user, err := user.Current()
	assert.Nil(err)
	_, err = New(SSHTypeNone, false, SSHConfig{Host: "127.0.0.2", User: user.Username})
	assert.NotNil(err)
	assert.Contains(err.Error(), "not found")
}

func TestLocalExecuteWithQuotes(t *testing.T) {
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))

	assert := require.New(t)
	user, err := user.Current()
	assert.Nil(err)
	local, err := New(SSHTypeNone, false, SSHConfig{Host: "127.0.0.1", User: user.Username})
	assert.Nil(err)

	deployDir, err := os.MkdirTemp("", "tiup-*")
	assert.Nil(err)
	defer os.RemoveAll(deployDir)

	cmds := []string{
		fmt.Sprintf(`find %s -type f -exec sed -i 's/\${DS_.*-CLUSTER}/hello/g' {} \;`, deployDir),
		fmt.Sprintf(`find %s -type f -exec sed -i 's/DS_.*-CLUSTER/hello/g' {} \;`, deployDir),
		`ls '/tmp'`,
	}
	for _, cmd := range cmds {
		_, _, err = local.Execute(ctx, cmd, false)
		assert.Nil(err)
	}
}
