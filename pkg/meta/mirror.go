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

package meta

type Manifest struct {
}

type Mirror interface {
	FetchManifest() *Manifest
	DownloadResource(uri string) (path string, err error)
}

type localFilesystem struct {
	rootPath string
}

// FetchManifest implements Mirror
func (l localFilesystem) FetchManifest() *Manifest {
	panic("implement me")
}

// DownloadResource implements Mirror
func (l localFilesystem) DownloadResource(uri string) (path string, err error) {
	panic("implement me")
}

type httpMirror struct {
	remote string
}

// FetchManifest implements Mirror
func (l httpMirror) FetchManifest() *Manifest {
	panic("implement me")
}

// DownloadResource implements Mirror
func (l httpMirror) DownloadResource(uri string) (path string, err error) {
	panic("implement me")
}
