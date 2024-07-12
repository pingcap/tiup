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
	"testing"
	"time"

	"github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type utilSuite struct{}

var _ = check.Suite(&utilSuite{})

func (s utilSuite) TestAbs(c *check.C) {
	var path string
	path = Abs(" foo", "")
	c.Assert(path, check.Equals, "/home/foo")
	path = Abs("foo ", " ")
	c.Assert(path, check.Equals, "/home/foo")
	path = Abs("foo", "bar")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", " bar")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", "bar ")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", " bar ")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", "/bar")
	c.Assert(path, check.Equals, "/bar")
	path = Abs("foo", " /bar")
	c.Assert(path, check.Equals, "/bar")
	path = Abs("foo", "/bar ")
	c.Assert(path, check.Equals, "/bar")
	path = Abs("foo", " /bar ")
	c.Assert(path, check.Equals, "/bar")
}

func (s *utilSuite) TestMultiDirAbs(c *check.C) {
	paths := MultiDirAbs("tidb", "")
	c.Assert(len(paths), check.Equals, 0)

	paths = MultiDirAbs("tidb", " ")
	c.Assert(len(paths), check.Equals, 0)

	paths = MultiDirAbs("tidb", "a ")
	c.Assert(len(paths), check.Equals, 1)
	c.Assert(paths[0], check.Equals, "/home/tidb/a")

	paths = MultiDirAbs("tidb", "a , /tmp/b")
	c.Assert(len(paths), check.Equals, 2)
	c.Assert(paths[0], check.Equals, "/home/tidb/a")
	c.Assert(paths[1], check.Equals, "/tmp/b")
}

func TestExtractScriptPath(t *testing.T) {
	command := "/bin/bash -c '/root/tidb-deploy/scheduling-3399/scripts/run_scheduling.sh'\n"
	scriptPath := extractScriptPath(command)
	println("scriptPath:", scriptPath)
	assert.Equal(t, "/root/tidb-deploy/scheduling-3399/scripts/run_scheduling.sh", scriptPath)
}

// MockExecutor simulates command execution
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error) {
	args := m.Called(ctx, cmd, sudo)
	return args.Get(0).([]byte), args.Get(1).([]byte), args.Error(2)
}

func (m *MockExecutor) Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) error {
	panic("implement me")
}

func TestModifyStartScriptPath(t *testing.T) {
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))
	component := "test-component"
	host := "localhost"
	port := 8080
	content := fmt.Sprintf(" \\\n    --name='%s'", "scheudling-0")

	// Setup temporary file for testing
	assert := require.New(t)
	conf, err := os.CreateTemp("", "scheduling.conf")
	defer os.Remove(conf.Name())
	assert.Nil(err)
	scriptPath := conf.Name()
	err = os.WriteFile(scriptPath, []byte(content), 0644)
	assert.Nil(err)

	// Mock executor
	mockExec := new(MockExecutor)
	shPath := fmt.Sprintf("/bin/bash -c '%s'", conf.Name())
	mockExec.On("Execute", ctx, mock.Anything, false).Return([]byte(shPath), []byte(""), nil)

	// Inject mock executor
	ctxt.GetInner(ctx).SetExecutor(host, mockExec)

	// Test function
	err = ModifyStartScriptPath(ctx, component, host, port, content)
	assert.Nil(err)

	// Verify file content
	resultContent, err := os.ReadFile(scriptPath)
	assert.Nil(err)
	assert.Contains(string(resultContent), content)
}
