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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// FsManifests represents a collection of v1 manifests on disk.
type FsManifests struct {
	root string
	keys *KeyStore
}

// FIXME implement garbage collection of old manifests

// NewManifests creates a new FsManifests with local store at root.
func NewManifests(root string) *FsManifests {
	return &FsManifests{root: root}
}

// LocalManifests methods for accessing a store of manifests.
type LocalManifests interface {
	// SaveManifest saves a manifest to disk, it will overwrite any unversioned manifest of the same type
	// and any existing manifest with the same type and version number.
	SaveManifest(manifest *Manifest) error
	// LoadManifest loads and validates the most recent manifest of role's type.
	LoadManifest(role ValidManifest) error
	// LoadComponentManifest loads and validates the most recent manifest for a component with the given id.
	LoadComponentManifest(id string) (*Component, error)
	// Keys returns the key store derived from these manifests.
	Keys() *KeyStore
}

// SaveManifest implements LocalManifests.
func (ms *FsManifests) SaveManifest(manifest *Manifest) error {
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

// LoadManifest implements LocalManifests.
func (ms *FsManifests) LoadManifest(role ValidManifest) error {
	return ms.load(role.Filename(), role)
}

// LoadComponentManifest implements LocalManifests.
func (ms *FsManifests) LoadComponentManifest(id string) (*Component, error) {
	var role Component
	return &role, ms.load(fmt.Sprintf("%s.json", id), &role)
}

// load and validate a manifest from disk.
func (ms *FsManifests) load(filename string, role ValidManifest) error {
	fullPath := filepath.Join(ms.root, filename)
	file, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = ReadManifest(file, role, ms.keys)
	return err
}

// Keys implements LocalManifests.
func (ms *FsManifests) Keys() *KeyStore {
	return ms.keys
}

// MockManifests is a LocalManifests implementation for testing.
type MockManifests struct {
	Manifests map[string]ValidManifest
}

// SaveManifest implements LocalManifests.
func (ms *MockManifests) SaveManifest(manifest *Manifest) error {
	return nil
}

// LoadManifest implements LocalManifests.
func (ms *MockManifests) LoadManifest(role ValidManifest) error {
	manifest, ok := ms.Manifests[role.Filename()]
	if !ok {
		return fmt.Errorf("No manifest for %s in mock manifests", role.Filename())
	}

	switch role.Filename() {
	case "root.json":
		ptr := role.(*Root)
		*ptr = *manifest.(*Root)
	case "index.json":
		ptr := role.(*Index)
		*ptr = *manifest.(*Index)
	case "snapshot.json":
		ptr := role.(*Snapshot)
		*ptr = *manifest.(*Snapshot)
	case "timestamp.json":
		ptr := role.(*Timestamp)
		*ptr = *manifest.(*Timestamp)
	default:
		return fmt.Errorf("Unknown manifest type: %s", role.Filename())
	}
	return nil
}

// LoadComponentManifest implements LocalManifests.
func (ms *MockManifests) LoadComponentManifest(id string) (*Component, error) {
	// TODO implement this
	return nil, nil
}

// Keys implements LocalManifests.
func (ms *MockManifests) Keys() *KeyStore {
	return nil
}
