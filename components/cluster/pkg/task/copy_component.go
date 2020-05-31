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

	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
)

// CopyComponent is used to copy all files related the specific version a component
// to the target directory of path
type CopyComponent struct {
	component string
	os        string
	arch      string
	version   string
	host      string
	dstDir    string
}

// Execute implements the Task interface
func (c *CopyComponent) Execute(ctx *Context) error {
	// Copy to remote server
	resName := fmt.Sprintf("%s-%s", c.component, c.version)
	fileName := fmt.Sprintf("%s-%s-%s.tar.gz", resName, c.os, c.arch)
	srcPath := meta.ProfilePath(meta.TiOpsPackageCacheDir, fileName)

	install := &InstallPackage{
		srcPath: srcPath,
		host:    c.host,
		dstDir:  c.dstDir,
	}

	return install.Execute(ctx)
}

// Rollback implements the Task interface
func (c *CopyComponent) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CopyComponent) String() string {
	return fmt.Sprintf("CopyComponent: component=%s, version=%s, remote=%s:%s os=%s, arch=%s",
		c.component, c.version, c.host, c.dstDir, c.os, c.arch)
}
