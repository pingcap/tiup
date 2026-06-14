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

package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/require"
)

type fakeShellExecutor struct {
	command string
}

func (e *fakeShellExecutor) Execute(_ context.Context, cmd string, _ bool, _ ...time.Duration) ([]byte, []byte, error) {
	e.command = cmd
	if strings.HasSuffix(cmd, "|| true") {
		return nil, nil, nil
	}
	return nil, nil, errors.New("command failed")
}

func (e *fakeShellExecutor) Transfer(_ context.Context, _, _ string, _ bool, _ int, _ bool) error {
	return nil
}

func TestShellIgnoreNonZeroWrapsCommand(t *testing.T) {
	exec := &fakeShellExecutor{}
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))
	ctxt.GetInner(ctx).SetExecutor("n1", exec)

	err := NewBuilder(nil).ShellIgnoreNonZero("n1", "missing-command", "", true).Build().Execute(ctx)
	require.NoError(t, err)
	require.Equal(t, "(missing-command) || true", exec.command)
}

func TestShellPropagatesCommandFailureByDefault(t *testing.T) {
	exec := &fakeShellExecutor{}
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))
	ctxt.GetInner(ctx).SetExecutor("n1", exec)

	err := NewBuilder(nil).Shell("n1", "missing-command", "", true).Build().Execute(ctx)
	require.Error(t, err)
	require.Equal(t, "missing-command", exec.command)
}
