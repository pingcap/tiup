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

package localdata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pingcap-incubator/tiup/pkg/repository"
)

// Manifests represents a collection of v1 manifests on disk.
type Manifests struct {
	root string
	keys *repository.KeyStore
}

// FIXME implement garbage collection of old manifests

// NewManifests creates a new Manifests with local store at root.
func NewManifests(root string) *Manifests {
	return &Manifests{root: root}
}

// SaveManifest saves a manifest to disk, it will overwrite any unversioned manifest of the same type
// and any existing manifest with the same type and version number.
func (ms *Manifests) SaveManifest(manifest *repository.Manifest) error {
	filename := manifest.Signed.Base().Filename()
	bytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	if manifest.Signed.Base().Versioned() {
		version := manifest.Signed.Base().Version
		err = ioutil.WriteFile(filepath.Join(ms.root, fmt.Sprintf("%v.%s", version, filename)), bytes, 0644)
		if err != nil {
			return err
		}
	}

	return ioutil.WriteFile(filepath.Join(ms.root, filename), bytes, 0644)
}

// LoadManifest loads and validates the most recent manifest of role's type.
func (ms *Manifests) LoadManifest(role repository.ValidManifest) error {
	return ms.load(role.Filename(), role)
}

// LoadComponentManifest loads and validates the most recent manifest for a component with the given id.
func (ms *Manifests) LoadComponentManifest(id string) (*repository.Component, error) {
	var role repository.Component
	return &role, ms.load(fmt.Sprintf("%s.json", id), &role)
}

// load and validate a manifest from disk.
func (ms *Manifests) load(filename string, role repository.ValidManifest) error {
	fullPath := filepath.Join(ms.root, filename)
	file, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return repository.ReadManifest(file, role, ms.keys)
}
