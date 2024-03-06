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

// Mkdir is used to create directory on the target host
type Mkdir struct {
	user string
	host string
	dirs []string
	sudo bool
}

// Execute implements the Task interface
func (m *Mkdir) Execute(ctx context.Context) error {
	exec, found := ctxt.GetInner(ctx).GetExecutor(m.host)
	if !found {
		panic(ErrNoExecutor)
	}
	for _, dir := range m.dirs {
		if !strings.HasPrefix(dir, "/") {
			return fmt.Errorf("dir is a relative path: %s", dir)
		}
		if strings.Contains(dir, ",") {
			return fmt.Errorf("dir name contains invalid characters: %v", dir)
		}

		xs := strings.Split(dir, "/")
		// Create directories recursively
		// The directory /a/b/c will be flatten to:
		// 		test -d /a || (mkdir /a && chown tidb:tidb /a)
		//		test -d /a/b || (mkdir /a/b && chown tidb:tidb /a/b)
		//		test -d /a/b/c || (mkdir /a/b/c && chown tidb:tidb /a/b/c)
		for i := 0; i < len(xs); i++ {
			if xs[i] == "" {
				continue
			}

			cmd := ""
			if m.sudo {
				cmd = fmt.Sprintf(
					`test -d %[1]s || (mkdir -p %[1]s && chown %[2]s:$(id -g -n %[2]s) %[1]s)`,
					strings.Join(xs[:i+1], "/"),
					m.user,
				)
			} else {
				cmd = fmt.Sprintf(
					`test -d %[1]s || (mkdir -p %[1]s)`,
					strings.Join(xs[:i+1], "/"),
				)
			}

			_, _, err := exec.Execute(ctx, cmd, m.sudo) // use root to create the dir
			if err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

// Rollback implements the Task interface
func (m *Mkdir) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (m *Mkdir) String() string {
	return fmt.Sprintf("Mkdir: host=%s, directories='%s'", m.host, strings.Join(m.dirs, "','"))
}
