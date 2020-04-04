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

	"github.com/pingcap-incubator/tiup-cluster/pkg/bindversion"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap/errors"
)

// BackupComponent is used to copy all files related the specific version a component
// to the target directory of path
type BackupComponent struct {
	component string
	fromVer   string
	host      string
	dstDir    string

	cpFrom string
	cpTo   string
}

// Execute implements the Task interface
func (c *BackupComponent) Execute(ctx *Context) error {
	m, found := ctx.GetManifest(c.component)
	if !found {
		manifest, err := meta.Repository().ComponentVersions(c.component)
		if err != nil {
			return err
		}
		m = manifest
		ctx.SetManifest(c.component, m)
	}

	// Copy to remote server
	exec, found := ctx.GetExecutor(c.host)
	if !found {
		return ErrNoExecutor
	}

	// backup old version
	var versionInfo repository.VersionInfo
	var foundVersion bool
	fromVer := bindversion.ComponentVersion(c.component, c.fromVer)
	for _, vi := range m.Versions {
		if vi.Version == fromVer {
			versionInfo = vi
			foundVersion = true
			break
		}
	}
	if !fromVer.IsNightly() && !foundVersion {
		return errors.Errorf("cannot found previous version %v in %s manifest", c.fromVer, c.component)
	}
	if fromVer.IsNightly() && m.Nightly == nil {
		return errors.Errorf("nightly version unsupported for component %s", c.component)
	}
	if fromVer.IsNightly() {
		versionInfo = *m.Nightly
	}

	dstDir := filepath.Join(c.dstDir, "bin")
	dstPathOld := filepath.Join(dstDir, versionInfo.Entry+".old")
	dstPath := filepath.Join(dstDir, versionInfo.Entry)

	c.cpFrom = dstPath
	c.cpTo = dstPathOld

	cmd := fmt.Sprintf(`cp %s %s`, dstPath, dstPathOld)
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
		c.component, c.fromVer, c.host, c.dstDir)
}
