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
	"bytes"
	"context"
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/pingcap/errors"
)

// Local execute the command at local host.
type Local struct {
}

var _ Executor = &Local{}

// Execute implements Executor interface.
func (l *Local) Execute(cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error) {
	ctx := context.Background()
	var cancel context.CancelFunc
	if len(timeout) > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout[0])
		defer cancel()
	}

	command := exec.CommandContext(ctx, "bash", "-c", cmd)

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	command.Stdout = stdoutBuf
	command.Stderr = stderrBuf

	err = command.Run()
	stdout = stderrBuf.Bytes()
	stderr = stderrBuf.Bytes()
	return
}

// Transfer implements Executer interface.
func (l *Local) Transfer(src string, dst string, download bool) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return errors.AddStack(err)
	}

	err = ioutil.WriteFile(dst, data, 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}
