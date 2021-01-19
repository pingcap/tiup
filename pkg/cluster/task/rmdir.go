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
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
)

// Rmdir is used to delete directory on the target host
type Rmdir struct {
	host string
	dirs []string
}

// Execute implements the Task interface
func (r *Rmdir) Execute(ctx context.Context) error {
	exec, found := ctxt.GetInner(ctx).GetExecutor(r.host)
	if !found {
		return ErrNoExecutor
	}

	cmd := fmt.Sprintf(`rm -rf %s`, strings.Join(r.dirs, " "))
	_, _, err := exec.Execute(ctx, cmd, false)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Rollback implements the Task interface
func (r *Rmdir) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (r *Rmdir) String() string {
	return fmt.Sprintf("Rmdir: host=%s, directories='%s'", r.host, strings.Join(r.dirs, "','"))
}
