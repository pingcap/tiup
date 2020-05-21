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

package v1manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/pingcap-incubator/tiup/pkg/utils"
)

// FsManifests represents a collection of v1 manifests on disk.
// Invariant: any manifest written to disk should be valid, but may have expired. (It is also possible the manifest was
// ok when written and has expired since).
type FsManifests struct {
	profile *localdata.Profile
	keys    crypto.KeyStore
}

// FIXME implement garbage collection of old manifests

// NewManifests creates a new FsManifests with local store at root.
// There must exist the trusted root.json.
// There must exists the trusted root.json.
func NewManifests(profile *localdata.Profile) *FsManifests {
	return &FsManifests{profile: profile}
}

// LocalManifests methods for accessing a store of manifests.
type LocalManifests interface {
	// SaveManifest saves a manifest to disk, it will overwrite filename if it exists.
	SaveManifest(manifest *Manifest, filename string) error
	// SaveComponentManifest saves a component manifest to disk, it will overwrite filename if it exists.
	SaveComponentManifest(manifest *Manifest, filename string) error
	// LoadManifest loads and validates the most recent manifest of role's type. The returned bool is true if the file
	// exists.
	LoadManifest(role ValidManifest) (bool, error)
	// LoadComponentManifest loads and validates the most recent manifest at filename.
	LoadComponentManifest(filename string) (*Component, error)
	// ComponentInstalled is true if the version of component is present locally.
	ComponentInstalled(component, version string) (bool, error)
	// InstallComponent installs the component from the reader.
	InstallComponent(reader io.Reader, component, version string) error
}

// SaveManifest implements LocalManifests.
func (ms *FsManifests) SaveManifest(manifest *Manifest, filename string) error {
	bytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(ms.profile.Root(), filename), bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

// SaveComponentManifest implements LocalManifests.
func (ms *FsManifests) SaveComponentManifest(manifest *Manifest, filename string) error {
	bytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(ms.profile.Root(), filename), bytes, 0644)
}

// LoadManifest implements LocalManifests.
func (ms *FsManifests) LoadManifest(role ValidManifest) (bool, error) {
	return ms.load(role.Filename(), role)
}

// LoadComponentManifest implements LocalManifests.
func (ms *FsManifests) LoadComponentManifest(filename string) (*Component, error) {
	var role Component
	exists, err := ms.load(filename, &role)
	if !exists {
		return nil, nil
	}
	return &role, err
}

// load and validate a manifest from disk. The returned bool is true if the file exists.
func (ms *FsManifests) load(filename string, role ValidManifest) (bool, error) {
	fullPath := filepath.Join(ms.profile.Root(), filename)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return true, err
	}
	defer file.Close()

	_, err = ReadManifest(file, role, nil)
	return true, err
}

// Keys implements LocalManifests.
func (ms *FsManifests) Keys() crypto.KeyStore {
	return ms.keys
}

// ComponentInstalled implements LocalManifests.
func (ms *FsManifests) ComponentInstalled(component, version string) (bool, error) {
	return ms.profile.VersionIsInstalled(component, version)
}

// InstallComponent implements LocalManifests.
func (ms *FsManifests) InstallComponent(reader io.Reader, component, version string) error {
	// TODO factor path construction to profile (also used by v0 repo).
	targetDir := ms.profile.Path(localdata.ComponentParentDir, component, version)
	// TODO handle disable decompression by writing directly to disk using the url as filename
	return utils.Untar(reader, targetDir)
}

// MockManifests is a LocalManifests implementation for testing.
type MockManifests struct {
	Manifests map[string]ValidManifest
	Saved     []string
}

// NewMockManifests creates an empty MockManifests.
func NewMockManifests() *MockManifests {
	return &MockManifests{
		Manifests: map[string]ValidManifest{},
		Saved:     []string{},
	}
}

// SaveManifest implements LocalManifests.
func (ms *MockManifests) SaveManifest(manifest *Manifest, filename string) error {
	ms.Saved = append(ms.Saved, filename)
	ms.Manifests[filename] = manifest.Signed
	return nil
}

// SaveComponentManifest implements LocalManifests.
func (ms *MockManifests) SaveComponentManifest(manifest *Manifest, filename string) error {
	ms.Saved = append(ms.Saved, filename)
	ms.Manifests[filename] = manifest.Signed
	return nil
}

// LoadManifest implements LocalManifests.
func (ms *MockManifests) LoadManifest(role ValidManifest) (bool, error) {
	manifest, ok := ms.Manifests[role.Filename()]
	if !ok {
		return false, nil
	}

	switch role.Filename() {
	case ManifestFilenameRoot:
		ptr := role.(*Root)
		*ptr = *manifest.(*Root)
	case ManifestFilenameIndex:
		ptr := role.(*Index)
		*ptr = *manifest.(*Index)
	case ManifestFilenameSnapshot:
		ptr := role.(*Snapshot)
		*ptr = *manifest.(*Snapshot)
	case ManifestFilenameTimestamp:
		ptr := role.(*Timestamp)
		*ptr = *manifest.(*Timestamp)
	default:
		return true, fmt.Errorf("unknown manifest type: %s", role.Filename())
	}
	return true, nil
}

// LoadComponentManifest implements LocalManifests.
func (ms *MockManifests) LoadComponentManifest(filename string) (*Component, error) {
	manifest, ok := ms.Manifests[filename]
	if !ok {
		return nil, nil
	}
	comp, ok := manifest.(*Component)
	if !ok {
		return nil, fmt.Errorf("manifest %s is not a component manifest", filename)
	}
	return comp, nil
}

// Keys implements LocalManifests.
func (ms *MockManifests) Keys() crypto.KeyStore {
	return nil
}

// ComponentInstalled implements LocalManifests.
func (ms *MockManifests) ComponentInstalled(component, version string) (bool, error) {
	return false, nil
}

// InstallComponent implements LocalManifests.
func (ms *MockManifests) InstallComponent(reader io.Reader, component, version string) error {
	return nil
}
