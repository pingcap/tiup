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

		err := repo.DownloadFile(meta.ProfilePath(meta.TiOpsPackageCacheDir), resName)
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
