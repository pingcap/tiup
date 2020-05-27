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
	"strings"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

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
	LoadComponentManifest(item *ComponentItem, filename string) (*Component, error)
	// ComponentInstalled is true if the version of component is present locally.
	ComponentInstalled(component, version string) (bool, error)
	// InstallComponent installs the component from the reader.
	InstallComponent(reader io.Reader, targetDir, component, version, filename string, noExpand bool) error
	// Return the local key store.
	KeyStore() *KeyStore
	// ManifestVersion opens filename, if it exists and is a manifest, returns its manifest version number. Otherwise
	// returns 0.
	ManifestVersion(filename string) uint
}

// FsManifests represents a collection of v1 manifests on disk.
// Invariant: any manifest written to disk should be valid, but may have expired. (It is also possible the manifest was
// ok when written and has expired since).
type FsManifests struct {
	profile *localdata.Profile
	keys    *KeyStore
	cache   map[string]string
}

// FIXME implement garbage collection of old manifests

// NewManifests creates a new FsManifests with local store at root.
// There must exist a trusted root.json.
func NewManifests(profile *localdata.Profile) (*FsManifests, error) {
	result := &FsManifests{profile: profile, keys: NewKeyStore(), cache: make(map[string]string)}

	// Load the root manifest.
	manifest, err := result.load(ManifestFilenameRoot)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	// We must load without validation because we have no keys yet.
	var root Root
	err = ReadNoVerify(strings.NewReader(manifest), &root)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	// Populate our key store from the root manifest.
	err = loadKeys(&root, result.keys)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	// Now that we've bootstrapped the key store, we can verify the root manifest we loaded earlier.
	_, err = ReadManifest(strings.NewReader(manifest), &root, result.keys)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	result.cache[ManifestFilenameRoot] = manifest

	return result, nil
}

// SaveManifest implements LocalManifests.
func (ms *FsManifests) SaveManifest(manifest *Manifest, filename string) error {
	err := ms.save(manifest, filename)
	if err != nil {
		return err
	}
	return loadKeys(manifest.Signed, ms.keys)
}

// SaveComponentManifest implements LocalManifests.
func (ms *FsManifests) SaveComponentManifest(manifest *Manifest, filename string) error {
	return ms.save(manifest, filename)
}

func (ms *FsManifests) save(manifest *Manifest, filename string) error {
	bytes, err := cjson.Marshal(manifest)
	if err != nil {
		return err
	}

	// Save all manifests in `$TIUP_HOME/manifests`
	path := filepath.Join(ms.profile.Root(), localdata.ManifestParentDir, filename)

	// create sub directory if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.Trace(err)
	}

	err = ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		return err
	}

	ms.cache[filename] = string(bytes)
	return nil
}

// LoadManifest implements LocalManifests.
func (ms *FsManifests) LoadManifest(role ValidManifest) (bool, error) {
	filename := role.Filename()
	manifest, err := ms.load(filename)
	if err != nil || manifest == "" {
		return false, err
	}

	_, err = ReadManifest(strings.NewReader(manifest), role, ms.keys)
	if err != nil {
		return true, err
	}

	ms.cache[filename] = manifest
	return true, loadKeys(role, ms.keys)
}

// LoadComponentManifest implements LocalManifests.
func (ms *FsManifests) LoadComponentManifest(item *ComponentItem, filename string) (*Component, error) {
	manifest, err := ms.load(filename)
	if err != nil || manifest == "" {
		return nil, err
	}

	component := new(Component)
	_, err = ReadComponentManifest(strings.NewReader(manifest), component, item, ms.keys)
	if err != nil {
		return nil, err
	}

	ms.cache[filename] = manifest
	return component, nil
}

// load return the file for the manifest from disk.
// The returned string is empty if the file does not exist.
func (ms *FsManifests) load(filename string) (string, error) {
	str, cached := ms.cache[filename]
	if cached {
		return str, nil
	}

	fullPath := filepath.Join(ms.profile.Root(), localdata.ManifestParentDir, filename)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Use the hardcode root.json if there is no root.json currently
			if filename == ManifestFilenameRoot {
				initRoot, err := filepath.Abs(filepath.Join(ms.profile.Root(), "bin/root.json"))
				if err != nil {
					return "", errors.Trace(err)
				}
				bytes, err := ioutil.ReadFile(initRoot)
				if err != nil {
					return "", errors.Errorf("cannot open the initial root.json at %s", initRoot)
				}
				return string(bytes), nil
			}
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	builder := strings.Builder{}
	io.Copy(&builder, file)
	return builder.String(), nil
}

// ComponentInstalled implements LocalManifests.
func (ms *FsManifests) ComponentInstalled(component, version string) (bool, error) {
	return ms.profile.VersionIsInstalled(component, version)
}

// InstallComponent implements LocalManifests.
func (ms *FsManifests) InstallComponent(reader io.Reader, targetDir, component, version, filename string, noExpand bool) error {
	// TODO factor path construction to profile (also used by v0 repo).
	if targetDir == "" {
		targetDir = ms.profile.Path(localdata.ComponentParentDir, component, version)
	}

	if !noExpand {
		return utils.Untar(reader, targetDir)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return errors.Trace(err)
	}
	writer, err := os.OpenFile(filepath.Join(targetDir, filename), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return errors.Trace(err)
	}
	defer writer.Close()

	if _, err = io.Copy(writer, reader); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// KeyStore implements LocalManifests.
func (ms *FsManifests) KeyStore() *KeyStore {
	return ms.keys
}

// ManifestVersion implements LocalManifests.
func (ms *FsManifests) ManifestVersion(filename string) uint {
	data, err := ms.load(filename)
	if err != nil {
		return 0
	}

	var manifest RawManifest
	err = json.Unmarshal([]byte(data), &manifest)
	if err != nil {
		return 0
	}

	var base SignedBase
	err = json.Unmarshal(manifest.Signed, &base)
	if err != nil {
		return 0
	}

	return base.Version
}

// MockManifests is a LocalManifests implementation for testing.
type MockManifests struct {
	Manifests map[string]ValidManifest
	Saved     []string
	Installed map[string]MockInstalled
	Ks        *KeyStore
}

// MockInstalled is used by MockManifests to remember what was installed for a component.
type MockInstalled struct {
	Version  string
	Contents string
}

// NewMockManifests creates an empty MockManifests.
func NewMockManifests() *MockManifests {
	return &MockManifests{
		Manifests: map[string]ValidManifest{},
		Saved:     []string{},
		Installed: map[string]MockInstalled{},
		Ks:        NewKeyStore(),
	}
}

// SaveManifest implements LocalManifests.
func (ms *MockManifests) SaveManifest(manifest *Manifest, filename string) error {
	ms.Saved = append(ms.Saved, filename)
	ms.Manifests[filename] = manifest.Signed
	loadKeys(manifest.Signed, ms.Ks)
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

	loadKeys(role, ms.Ks)
	return true, nil
}

// LoadComponentManifest implements LocalManifests.
func (ms *MockManifests) LoadComponentManifest(item *ComponentItem, filename string) (*Component, error) {
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

// ComponentInstalled implements LocalManifests.
func (ms *MockManifests) ComponentInstalled(component, version string) (bool, error) {
	inst, ok := ms.Installed[component]
	if !ok {
		return false, nil
	}

	return inst.Version == version, nil
}

// InstallComponent implements LocalManifests.
func (ms *MockManifests) InstallComponent(reader io.Reader, targetDir string, component, version, filename string, noExpand bool) error {
	buf := strings.Builder{}
	io.Copy(&buf, reader)
	ms.Installed[component] = MockInstalled{
		Version:  version,
		Contents: buf.String(),
	}
	return nil
}

// KeyStore implements LocalManifests.
func (ms *MockManifests) KeyStore() *KeyStore {
	return ms.Ks
}

// ManifestVersion implements LocalManifests.
func (ms *MockManifests) ManifestVersion(filename string) uint {
	manifest, ok := ms.Manifests[filename]
	if ok {
		return manifest.Base().Version
	}
	return 0
}
