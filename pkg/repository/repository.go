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

package repository

import (
	"fmt"
	"runtime"
)

// Repository represents a components repository. All logic concerning manifests and the locations of tarballs
// is contained in the Repository object. Any IO is delegated to mirrorSource, which in turn will delegate fetching
// files to a Mirror.
type Repository struct {
	Options
	mirror     Mirror
	fileSource fileSource
}

// Options represents options for a repository
type Options struct {
	SkipVersionCheck  bool
	GOOS              string
	GOARCH            string
	DisableDecompress bool
}

// NewRepository returns a repository instance based on mirror. mirror should be in an open state.
func NewRepository(mirror Mirror, opts Options) (*Repository, error) {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	fileSource := &mirrorSource{mirror: mirror}
	if err := fileSource.open(); err != nil {
		return nil, err
	}

	return &Repository{
		Options:    opts,
		mirror:     mirror,
		fileSource: fileSource,
	}, nil
}

// Close shuts down the repository, including any open mirrors.
func (r *Repository) Close() error {
	return r.fileSource.close()
}

// Mirror returns the mirror of the repository
func (r *Repository) Mirror() Mirror {
	return r.mirror
}

// DownloadTiUP downloads the tiup tarball and expands it into targetDir
func (r *Repository) DownloadTiUP(targetDir string) error {
	return r.fileSource.downloadTarFile(targetDir, fmt.Sprintf("%s-%s-%s", TiUPBinaryName, r.GOOS, r.GOARCH), true)
}
