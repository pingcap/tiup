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
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
)

// Downloader is used to download the specific version of a component from
// the repository, there is nothing to do if the specified version exists.
type Downloader struct {
	component string
	version   repository.Version
}

// Execute implements the Task interface
func (d *Downloader) Execute(_ *Context) error {
	return meta.DownloadComponent(d.component, d.version, false)
}

// Rollback implements the Task interface
func (d *Downloader) Rollback(ctx *Context) error {
	// We cannot delete the component because of some versions maybe exists before
	return nil
}
