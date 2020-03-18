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
	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap/errors"
)

// SSH is used to establish a SSH connection to the target host with specific key
type SSH struct {
	host    string
	keypath string
	user    string
}

// Execute implements the Task interface
func (s SSH) Execute(ctx *Context) error {
	e, err := executor.NewSSHExecutor(executor.SSHConfig{
		Host:    s.host,
		KeyFile: s.keypath,
		User:    s.user,
	})

	if err != nil {
		return errors.Trace(err)
	}

	ctx.SetExecutor(s.host, e)
	return nil
}

// Rollback implements the Task interface
func (s SSH) Rollback(ctx *Context) error {
	ctx.exec.Lock()
	delete(ctx.exec.executors, s.host)
	ctx.exec.Unlock()
	return nil
}
