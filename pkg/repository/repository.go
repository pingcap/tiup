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
	"github.com/pingcap/errors"
	"path/filepath"
	"runtime"
)

// Repository represents a components repository. All logic concerning manifests and the locations of tarballs
// is contained in the Repository object. Any IO is delegated to mirrorSource, which in turn will delegate fetching
// files to a Mirror.
type Repository struct {
	fileSource fileSource
	opts       Options
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

	return &Repository{fileSource: fileSource, opts: opts}, nil
}

// Close shuts down the repository, including any open mirrors.
func (r *Repository) Close() error {
	return r.fileSource.close()
}

// Manifest returns the component manifest fetched from repository
func (r *Repository) Manifest() (*ComponentManifest, error) {
	var manifest ComponentManifest
	err := r.fileSource.downloadJSON(ManifestFileName, &manifest)
	return &manifest, err
}

// ComponentVersions returns the version manifest of specific component
func (r *Repository) ComponentVersions(component string) (*VersionManifest, error) {
	var vers VersionManifest
	err := r.fileSource.downloadJSON(fmt.Sprintf("tiup-component-%s.index", component), &vers)
	if err != nil {
		return nil, errors.Trace(err)
	}
	vers.sort()
	return &vers, nil
}

// DownloadTiup downloads the tiup tarball and expands it into targetDir
func (r *Repository) DownloadTiup(targetDir string) error {
	return r.fileSource.downloadTarFile(targetDir, TiupBinaryName, true)
}

// DownloadComponent downloads a component with specific version from repository
// support `<component>[:version]` format
func (r *Repository) DownloadComponent(compsDir, component string, version Version) error {
	versions, err := r.ComponentVersions(component)
	if err != nil {
		return err
	}
	if version.IsEmpty() {
		version = versions.LatestVersion()
	} else if !version.IsNightly() {
		if !versions.ContainsVersion(version) {
			return fmt.Errorf("component `%s` doesn't release the version `%s`", component, version)
		}
	}
	if !r.opts.SkipVersionCheck && !version.IsNightly() && !version.IsValid() {
		return errors.Errorf("invalid version `%s`", version)
	}
	resName := fmt.Sprintf("%s-%s", component, version)
	targetDir := filepath.Join(compsDir, component, version.String())
	return r.fileSource.downloadTarFile(targetDir, fmt.Sprintf("%s-%s-%s", resName, r.opts.GOOS, r.opts.GOARCH), !r.opts.DisableDecompress)
}
