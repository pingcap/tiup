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
	"path/filepath"

	"github.com/pingcap/errors"
)

// BackupComponent is used to copy all files related the specific version a component
// to the target directory of path
type BackupComponent struct {
	component string
	fromVer   string
	host      string
	deployDir string
}

// Execute implements the Task interface
func (c *BackupComponent) Execute(ctx *Context) error {
	// Copy to remote server
	exec, found := ctx.GetExecutor(c.host)
	if !found {
		return ErrNoExecutor
	}

	binDir := filepath.Join(c.deployDir, "bin")

	// Make upgrade idempotent
	// The old version has been backup if upgrade abort
	cmd := fmt.Sprintf(`test -d %[2]s || mv %[1]s %[2]s && mkdir -p %[1]s`, binDir, binDir+".old."+c.fromVer)
	_, _, err := exec.Execute(cmd, false)
	if err != nil {
		return errors.Annotate(err, cmd)
	}
	return nil
}

// Rollback implements the Task interface
func (c *BackupComponent) Rollback(ctx *Context) error {
	return nil
}

// String implements the fmt.Stringer interface
func (c *BackupComponent) String() string {
	return fmt.Sprintf("BackupComponent: component=%s, currentVersion=%s, remote=%s:%s",
		c.component, c.fromVer, c.host, c.deployDir)
}
