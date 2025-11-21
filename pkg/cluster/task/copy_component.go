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

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
)

// CopyComponent is used to copy all files related the specific version a component
// to the target directory of path
type CopyComponent struct {
	component string
	os        string
	arch      string
	version   string
	host      string
	srcPath   string
	dstDir    string
}

// Execute implements the Task interface
func (c *CopyComponent) Execute(ctx context.Context) error {
	// If the version is not specified, the last stable one will be used
	if c.version == "" {
		env := environment.GlobalEnv()
		ver, _, err := env.V1Repository().WithOptions(repository.Options{
			GOOS:   c.os,
			GOARCH: c.arch,
		}).LatestStableVersion(c.component, false, nil)
		if err != nil {
			return err
		}
		c.version = string(ver)
	}

	// Copy to remote server
	srcPath := c.srcPath
	if srcPath == "" {
		srcPath = spec.PackagePath(c.component, c.version, c.os, c.arch)
	}

	install := &InstallPackage{
		srcPath: srcPath,
		host:    c.host,
		dstDir:  c.dstDir,
	}

	return install.Execute(ctx)
}

// Rollback implements the Task interface
func (c *CopyComponent) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CopyComponent) String() string {
	return fmt.Sprintf("CopyComponent: component=%s, version=%s, remote=%s:%s os=%s, arch=%s",
		c.component, c.version, c.host, c.dstDir, c.os, c.arch)
}
