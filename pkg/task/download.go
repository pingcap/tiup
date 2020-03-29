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

	"github.com/pingcap-incubator/tiops/pkg/meta"
	tiupmeta "github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

// Downloader is used to download the specific version of a component from
// the repository, there is nothing to do if the specified version exists.
type Downloader struct {
	component string
	version   repository.Version
}

// Execute implements the Task interface
func (d *Downloader) Execute(_ *Context) error {
	if d.component == "" {
		return errors.New("component name not specified")
	}
	if d.version.IsEmpty() {
		return errors.Errorf("version not specified for component '%s'", d.component)
	}

	resName := fmt.Sprintf("%s-%s", d.component, d.version)
	fileName := fmt.Sprintf("%s-linux-amd64.tar.gz", resName)
	srcPath := meta.ProfilePath(meta.TiOpsPackageCacheDir, fileName)

	// Download from repository if not exists
	if utils.IsNotExist(srcPath) {
		mirror := repository.NewMirror(tiupmeta.Mirror())
		if err := mirror.Open(); err != nil {
			return errors.Trace(err)
		}
		defer mirror.Close()

		repo := repository.NewRepository(mirror, repository.Options{
			GOOS:              "linux",
			GOARCH:            "amd64",
			DisableDecompress: true,
		})

		versions, err := repo.ComponentVersions(d.component)
		if err != nil {
			return err
		}
		if !versions.ContainsVersion(d.version) {
			return errors.Errorf("component '%s' doesn't contains version '%s'", d.component, d.version)
		}

		err = repo.DownloadFile(meta.ProfilePath(meta.TiOpsPackageCacheDir), resName)
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

// Rollback implements the Task interface
func (d *Downloader) Rollback(ctx *Context) error {
	// We cannot delete the component because of some versions maybe exists before
	return nil
}

// String implements the fmt.Stringer interface
func (d *Downloader) String() string {
	return fmt.Sprintf("Download: component=%s, version=%s", d.component, d.version)
}
