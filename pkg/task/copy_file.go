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

import "github.com/pingcap/errors"

// CopyFile will copy a local file to the target host
type CopyFile struct {
	src     string
	dstHost string
	dstPath string
}

// Execute implements the Task interface
func (c *CopyFile) Execute(ctx *Context) error {
	e, ok := ctx.GetExecutor(c.dstHost)
	if !ok {
		return ErrNoExecutor
	}

	err := e.Transfer(c.src, c.dstPath)
	if err != nil {
		return errors.Annotate(err, "failed to transfer file")
	}

	return nil
}

// Rollback implements the Task interface
func (c *CopyFile) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}
