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
)

// CopyFile will copy a local file to the target host
type CopyFile struct {
	src      string
	dst      string
	remote   string
	download bool
	limit    int
	compress bool
}

// Execute implements the Task interface
func (c *CopyFile) Execute(ctx context.Context) error {
	e, ok := ctxt.GetInner(ctx).GetExecutor(c.remote)
	if !ok {
		return ErrNoExecutor
	}

	err := e.Transfer(ctx, c.src, c.dst, c.download, c.limit, c.compress)
	if err != nil {
		return errors.Annotate(err, "failed to transfer file")
	}

	return nil
}

// Rollback implements the Task interface
func (c *CopyFile) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CopyFile) String() string {
	if c.download {
		return fmt.Sprintf("CopyFile: remote=%s:%s, local=%s", c.remote, c.src, c.dst)
	}
	return fmt.Sprintf("CopyFile: local=%s, remote=%s:%s", c.src, c.remote, c.dst)
}
