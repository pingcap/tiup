// Copyright 2024 PingCAP, Inc.
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

package operator

import (
	"context"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockExecutor is a mock of Executor interface
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

func TestGetVersion(t *testing.T) {
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))
	deployDir := "/fake/deploy/dir"
	host := "localhost"
	expectedVersion := "v8.3.0-abcdef"
	mockExecutor := new(MockExecutor)
	mockExecutor.On("Execute", ctx, "/fake/deploy/dir/bin/pd-server --version", false).Return([]byte("Release Version: v8.3.0-abcdef\nEdition: Community\nGit Commit Hash: b235bc1152a7942d71d42222b0e078d91ccd0106\nGit Branch: heads/refs/tags/v8.3.0\nUTC Build Time:  2024-07-01 05:08:57"), []byte(""), nil)
	ctxt.GetInner(ctx).SetExecutor(host, mockExecutor)
	// Execute
	version, err := getVersion(ctx, deployDir, host)
	assert.NoError(t, err)
	assert.Equal(t, expectedVersion, version)

	expectedVersion = "v8.2.0-abcdef"
	mockExecutor = new(MockExecutor)
	mockExecutor.On("Execute", ctx, "/fake/deploy/dir/bin/pd-server --version", false).Return([]byte("Release Version: v8.2.0-abcdef\nEdition: Community\nGit Commit Hash: b235bc1152a7942d71d42222b0e078d91ccd0106\nGit Branch: heads/refs/tags/v8.3.0\nUTC Build Time:  2024-07-01 05:08:57"), []byte(""), nil)
	ctxt.GetInner(ctx).SetExecutor(host, mockExecutor)
	// Execute
	version, err = getVersion(ctx, deployDir, host)
	assert.NoError(t, err)
	assert.Equal(t, expectedVersion, version)

	expectedVersion = "nightly-abcdef"
	mockExecutor = new(MockExecutor)
	mockExecutor.On("Execute", ctx, "/fake/deploy/dir/bin/pd-server --version", false).Return([]byte("Release Version: nightly-abcdef\nEdition: Community\nGit Commit Hash: b235bc1152a7942d71d42222b0e078d91ccd0106\nGit Branch: heads/refs/tags/v8.3.0\nUTC Build Time:  2024-07-01 05:08:57"), []byte(""), nil)
	ctxt.GetInner(ctx).SetExecutor(host, mockExecutor)
	// Execute
	version, err = getVersion(ctx, deployDir, host)
	assert.NoError(t, err)
	assert.Equal(t, expectedVersion, version)

	// Teardown
	mockExecutor.AssertExpectations(t)
}
