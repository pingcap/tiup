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
	"path"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
)

// InstallPackage is used to copy all files related the specific version a component
// to the target directory of path
type InstallPackage struct {
	srcPath string
	host    string
	dstDir  string
}

// Execute implements the Task interface
func (c *InstallPackage) Execute(ctx context.Context) error {
	// Install package to remote server
	exec, found := ctxt.GetInner(ctx).GetExecutor(c.host)
	if !found {
		return ErrNoExecutor
	}

	dstDir := filepath.Join(c.dstDir, "bin")
	dstPath := filepath.Join(dstDir, path.Base(c.srcPath))

	err := exec.Transfer(ctx, c.srcPath, dstPath, false, 0, false)
	if err != nil {
		return errors.Annotatef(err, "failed to scp %s to %s:%s", c.srcPath, c.host, dstPath)
	}

	cmd := fmt.Sprintf(`tar --no-same-owner -zxf %s -C %s && rm %s`, dstPath, dstDir, dstPath)

	_, stderr, err := exec.Execute(ctx, cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}
	return nil
}

// Rollback implements the Task interface
func (c *InstallPackage) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *InstallPackage) String() string {
	return fmt.Sprintf("InstallPackage: srcPath=%s, remote=%s:%s", c.srcPath, c.host, c.dstDir)
}
