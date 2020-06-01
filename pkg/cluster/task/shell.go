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
	"fmt"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/logger/log"
)

// Shell is used to create directory on the target host
type Shell struct {
	host    string
	command string
	sudo    bool
}

// Execute implements the Task interface
func (m *Shell) Execute(ctx *Context) error {
	exec, found := ctx.GetExecutor(m.host)
	if !found {
		return ErrNoExecutor
	}

	log.Infof("Run command on %s(sudo:%v): %s", m.host, m.sudo, m.command)

	stdout, stderr, err := exec.Execute(m.command, m.sudo)
	ctx.SetOutputs(m.host, stdout, stderr)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Rollback implements the Task interface
func (m *Shell) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (m *Shell) String() string {
	return fmt.Sprintf("Shell: host=%s, sudo=%v, command=`%s`", m.host, m.sudo, m.command)
}
