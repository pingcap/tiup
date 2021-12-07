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

// Rmdir is used to delete directory on the target host
type Exist struct {
	host        string
	path        string
	isFile      bool
	isDirectory bool
}

// Execute implements the Task interface
func (e *Exist) Execute(ctx context.Context) error {
	exec, found := ctxt.GetInner(ctx).GetExecutor(e.host)
	if !found {
		return ErrNoExecutor
	}

	cmd := fmt.Sprintf(`[ -e %s ] && echo 1`, e.path)

	switch {
	case e.isFile:
		cmd = fmt.Sprintf(`[ -f %s ] && echo 1`, e.path)
	case e.isDirectory:
		cmd = fmt.Sprintf(`[ -d %s ] && echo 1`, e.path)
	}

	req, _, err := exec.Execute(ctx, cmd, false)
	if err != nil {
		return errors.Trace(err)
	}
	if string(req) != "1" {
		return fmt.Errorf("`%s` does not exist on the target host `%s`", e.path, e.host)
	}

	return nil
}

// Rollback implements the Task interface
func (e *Exist) Rollback(ctx context.Context) error {
	return nil
}

// String implements the fmt.Stringer interface
func (e *Exist) String() string {
	return fmt.Sprintf("Exist: host=%s, path='%s'", e.host, e.path)
}
