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
	"path/filepath"
	"runtime"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
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

// Manifest returns the v0 component manifest fetched from repository
func (r *Repository) Manifest() (*v0manifest.ComponentManifest, error) {
	var manifest v0manifest.ComponentManifest
	err := r.fileSource.downloadJSON(ManifestFileName, &manifest)
	return &manifest, err
}

// Mirror returns the mirror of the repository
func (r *Repository) Mirror() Mirror {
	return r.mirror
}

// ComponentVersions returns the version manifest of specific component
func (r *Repository) ComponentVersions(component string) (*v0manifest.VersionManifest, error) {
	var vers v0manifest.VersionManifest
	err := r.fileSource.downloadJSON(fmt.Sprintf("tiup-component-%s.index", component), &vers)
	if err != nil {
		return nil, errors.Trace(err)
	}
	vers.Sort()
	return &vers, nil
}

// LatestStableVersion returns the latest stable version of specific component
func (r *Repository) LatestStableVersion(component string) (v0manifest.Version, error) {
	ver, err := r.ComponentVersions(component)
	if err != nil {
		return "", err
	}
	return ver.LatestVersion(), nil
}

// DownloadTiup downloads the tiup tarball and expands it into targetDir
func (r *Repository) DownloadTiup(targetDir string) error {
	return r.fileSource.downloadTarFile(targetDir, fmt.Sprintf("%s-%s-%s", TiupBinaryName, r.GOOS, r.GOARCH), true)
}

// DownloadComponent downloads a component with specific version from repository
// support `<component>[:version]` format
func (r *Repository) DownloadComponent(compsDir, component string, version v0manifest.Version) error {
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
	if !r.SkipVersionCheck && !version.IsNightly() && !version.IsValid() {
		return errors.Errorf("invalid version `%s`", version)
	}
	resName := fmt.Sprintf("%s-%s", component, version)
	targetDir := filepath.Join(compsDir, component, version.String())
	return r.fileSource.downloadTarFile(targetDir, fmt.Sprintf("%s-%s-%s", resName, r.GOOS, r.GOARCH), !r.DisableDecompress)
}
