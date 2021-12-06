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
	"fmt"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
)

// Shell is used to create directory on the target host
type Shell struct {
	host    string
	command string
	sudo    bool
	cmdID   string
}

// Execute implements the Task interface
func (m *Shell) Execute(ctx context.Context) error {
	exec, found := ctxt.GetInner(ctx).GetExecutor(m.host)
	if !found {
		return ErrNoExecutor
	}

	ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger).
		Infof("Run command on %s(sudo:%v): %s", m.host, m.sudo, m.command)

	stdout, stderr, err := exec.Execute(ctx, m.command, m.sudo)
	outputID := m.host
	if m.cmdID != "" {
		outputID = m.cmdID
	}
	ctxt.GetInner(ctx).SetOutputs(outputID, stdout, stderr)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Rollback implements the Task interface
func (m *Shell) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (m *Shell) String() string {
	return fmt.Sprintf("Shell: host=%s, sudo=%v, command=`%s`", m.host, m.sudo, m.command)
}
