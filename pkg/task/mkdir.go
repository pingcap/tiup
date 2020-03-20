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
	"strings"

	"github.com/pingcap/errors"
)

// Mkdir is used to create directory on the target host
type Mkdir struct {
	host string
	dirs []string
}

// Execute implements the Task interface
func (m *Mkdir) Execute(ctx *Context) error {
	exec, found := ctx.GetExecutor(m.host)
	if !found {
		return ErrNoExecutor
	}

	cmd := fmt.Sprintf(`mkdir -p {%s}`, strings.Join(m.dirs, ","))
	fmt.Println("Create directories cmd: ", cmd)

	stdout, stderr, err := exec.Execute(cmd, false)
	if err != nil {
		return errors.Trace(err)
	}

	fmt.Println("Create directories stdout: ", string(stdout))
	fmt.Println("Create directories stderr: ", string(stderr))
	return nil
}

// Rollback implements the Task interface
func (m *Mkdir) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}
