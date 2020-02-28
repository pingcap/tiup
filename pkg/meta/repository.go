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

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

const (
	manifestFile = "tiup-manifest.index"
)

// Repository represents a components repository
type Repository struct {
	mirror Mirror
}

// NewRepository returns a repository instance base on mirror
func NewRepository(mirror Mirror) *Repository {
	return &Repository{mirror: mirror}
}

// Mirror returns the mirror which is used by repository
func (r *Repository) Mirror() Mirror {
	return r.mirror
}

// ReplaceMirror replaces the mirror
func (r *Repository) ReplaceMirror(mirror Mirror) error {
	err := r.mirror.Close()
	if err != nil {
		return err
	}

	r.mirror = mirror
	return r.mirror.Open()
}

// Manifest returns the component manifest fetched from repository
func (r *Repository) Manifest() (*ComponentManifest, error) {
	local, err := r.mirror.Fetch(manifestFile)
	if err != nil {
		return nil, errors.Trace(err)
	}

	file, err := os.OpenFile(local, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer file.Close()

	var manifest ComponentManifest
	err = json.NewDecoder(file).Decode(&manifest)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &manifest, nil
}

// ComponentVersions returns the version manifest of specific component
func (r *Repository) ComponentVersions(component string) (*VersionManifest, error) {
	local, err := r.mirror.Fetch(fmt.Sprintf("tiup-component-%s.index", component))
	if err != nil {
		return nil, errors.Trace(err)
	}

	file, err := os.OpenFile(local, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer file.Close()

	var vers VersionManifest
	err = json.NewDecoder(file).Decode(&vers)
	if err != nil {
		return nil, errors.Trace(err)
	}
	vers.sort()
	return &vers, nil
}

// Download downloads a component with specific version from repository
func (r *Repository) Download(compsDir, component string, version Version) error {
	if !version.IsValid() {
		return errors.Errorf("invalid version `%s`", version)
	}
	resName := fmt.Sprintf("%s-%s-%s-%s", component, version, runtime.GOOS, runtime.GOARCH)
	localPath, err := r.mirror.Fetch(resName + ".tar.gz")
	if err != nil {
		return errors.Trace(err)
	}

	sha1Path, err := r.mirror.Fetch(resName + ".sha1")
	if err != nil {
		return errors.Trace(err)
	}

	sha1Content, err := ioutil.ReadFile(sha1Path)
	if err != nil {
		return errors.Trace(err)
	}

	tarball, err := os.OpenFile(localPath, os.O_RDONLY, 0)
	if err != nil {
		return errors.Trace(err)
	}
	defer tarball.Close()

	sha1Writter := sha1.New()
	if _, err := io.Copy(sha1Writter, tarball); err != nil {
		return errors.Trace(err)
	}

	checksum := hex.EncodeToString(sha1Writter.Sum(nil))
	if checksum != strings.TrimSpace(string(sha1Content)) {
		return errors.Errorf("checksum mismatch, expect: %v, got: %v", string(sha1Content), checksum)
	}

	targetDir := filepath.Join(compsDir, component, version.String())
	if err := utils.CreateDir(targetDir); err != nil {
		return errors.Trace(err)
	}

	return utils.Untar(localPath, targetDir)
}
