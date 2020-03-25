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

	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap/errors"
)

// CopyComponent is used to copy all files related the specific version a component
// to the target directory of path
type CopyComponent struct {
	component string
	version   repository.Version
	host      string
	dstDir    string
}

// Execute implements the Task interface
func (c *CopyComponent) Execute(ctx *Context) error {
	// Copy to remote server
	exec, found := ctx.GetExecutor(c.host)
	if !found {
		return ErrNoExecutor
	}

	resName := fmt.Sprintf("%s-%s", c.component, c.version)
	fileName := fmt.Sprintf("%s-linux-amd64.tar.gz", resName)
	srcPath := meta.ProfilePath(meta.TiOpsPackageCacheDir, fileName)
	dstDir := filepath.Join(c.dstDir, "bin")
	dstPath := filepath.Join(dstDir, fileName)

	err := exec.Transfer(srcPath, dstPath)
	if err != nil {
		return errors.Trace(err)
	}

	cmd := fmt.Sprintf(`tar -xzf %s -C %s && rm %s`, dstPath, dstDir, dstPath)

	stdout, stderr, err := exec.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	fmt.Println("Decompress tarball stdout: ", string(stdout))
	fmt.Println("Decompress tarball stderr: ", string(stderr))

	return nil
}

// Rollback implements the Task interface
func (c *CopyComponent) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}
