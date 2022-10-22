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
	"os"
	"path/filepath"
	"strings"
	"sync"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/utils"
)

// LocalManifests methods for accessing a store of manifests.
type LocalManifests interface {
	// SaveManifest saves a manifest to disk, it will overwrite filename if it exists.
	SaveManifest(manifest *Manifest, filename string) error
	// SaveComponentManifest saves a component manifest to disk, it will overwrite filename if it exists.
	SaveComponentManifest(manifest *Manifest, filename string) error
	// LoadManifest loads and validates the most recent manifest of role's type. The returned bool is true if the file
	// exists.
	LoadManifest(role ValidManifest) (*Manifest, bool, error)
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

	// TargetRootDir returns the root directory of target
	TargetRootDir() string

	// copy from local profile

	// Path returns a full path which is related to profile root directory
	ProfilePath(relpath ...string) string
	// Root returns the root path of the `tiup`
	ProfileRoot() string
	ProfileName() string

	// GetComponentInstalledVersion return the installed version of component.
	GetComponentInstalledVersion(component string, ver utils.Version) (utils.Version, error)
	// ComponentInstalledPath returns the path where the component installed
	ComponentInstalledPath(component string, version utils.Version) (string, error)

	// InstalledComponents returns the installed components
	InstalledComponents() ([]string, error)
	// InstalledVersions returns the installed versions of specific component
	InstalledVersions(component string) ([]string, error)
	// VersionIsInstalled returns true if exactly version of component is installed.
	VersionIsInstalled(component, version string) (bool, error)
	// ResetMirror reset root.json and cleanup manifests directory
	ResetMirror(addr, root string) error
	// ReadMetaFile reads a Process object from dirName/MetaFilename. Returns (nil, nil) if a metafile does not exist.
	ReadMetaFile(dirName string) (*localdata.Process, error)

	Name() string
}

// FsManifests represents a collection of v1 manifests on disk.
// Invariant: any manifest written to disk should be valid, but may have expired. (It is also possible the manifest was
// ok when written and has expired since).
type FsManifests struct {
	profile *localdata.Profile
	keys    *KeyStore
	cache   sync.Map // map[string]string
}

// FIXME implement garbage collection of old manifests

// NewManifests creates a new FsManifests with local store at root.
// There must exist a trusted root.json.
func NewManifests(profile *localdata.Profile) (*FsManifests, error) {
	result := &FsManifests{profile: profile, keys: NewKeyStore()}

	// Load the root manifest.
	manifest, err := result.load(ManifestFilenameRoot)
	if err != nil {
		return nil, err
	}

	// We must load without validation because we have no keys yet.
	var root Root
	_, err = ReadNoVerify(strings.NewReader(manifest), &root)
	if err != nil {
		return nil, err
	}

	// Populate our key store from the root manifest.
	err = loadKeys(&root, result.keys)
	if err != nil {
		return nil, err
	}

	// Now that we've bootstrapped the key store, we can verify the root manifest we loaded earlier.
	_, err = ReadManifest(strings.NewReader(manifest), &root, result.keys)
	if err != nil {
		return nil, err
	}

	result.cache.Store(ManifestFilenameRoot, manifest)

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
	path := filepath.Join(ms.profile.Root(), localdata.ManifestParentDir, ms.profile.Name(), filename)

	// create sub directory if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.Trace(err)
	}

	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		return err
	}

	ms.cache.Store(filename, string(bytes))
	return nil
}

// LoadManifest implements LocalManifests.
func (ms *FsManifests) LoadManifest(role ValidManifest) (*Manifest, bool, error) {
	filename := role.Filename()
	manifest, err := ms.load(filename)
	if err != nil || manifest == "" {
		return nil, false, err
	}

	m, err := ReadManifest(strings.NewReader(manifest), role, ms.keys)
	if err != nil {
		// We can think that there is no such file if it's expired or has wrong signature
		if IsExpirationError(errors.Cause(err)) || IsSignatureError(errors.Cause(err)) {
			// Maybe we can os.Remove(filename) here
			if IsSignatureError(errors.Cause(err)) {
				fmt.Printf("Warn: %s\n", err.Error())
			}
			return nil, false, nil
		}
		return m, true, err
	}

	ms.cache.Store(filename, manifest)
	return m, true, loadKeys(role, ms.keys)
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
		// We can think that there is no such file if it's expired or has wrong signature
		if IsExpirationError(errors.Cause(err)) || IsSignatureError(errors.Cause(err)) {
			// Maybe we can os.Remove(filename) here
			if IsSignatureError(errors.Cause(err)) {
				fmt.Printf("Warn: %s\n", err.Error())
			}
			return nil, nil
		}
		return nil, err
	}

	ms.cache.Store(filename, manifest)
	return component, nil
}

// load return the file for the manifest from disk.
// The returned string is empty if the file does not exist.
func (ms *FsManifests) load(filename string) (string, error) {
	str, cached := ms.cache.Load(filename)
	if cached {
		return str.(string), nil
	}

	fullPath := filepath.Join(ms.profile.Root(), localdata.ManifestParentDir, ms.profile.Name(), filename)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Use the hardcode root.json if there is no root.json currently
			if filename == ManifestFilenameRoot {
				initRoot, err := filepath.Abs(filepath.Join(ms.profile.Root(), localdata.TrustedDir, ms.Name(), "root.json"))
				if err != nil {
					return "", errors.Trace(err)
				}
				bytes, err := os.ReadFile(initRoot)
				if err != nil {
					return "", &LoadManifestError{
						manifest: "root.json",
						err:      err,
					}
				}
				return string(bytes), nil
			}
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	builder := strings.Builder{}
	_, err = io.Copy(&builder, file)
	if err != nil {
		return "", errors.AddStack(err)
	}

	return builder.String(), nil
}

// ComponentInstalled implements LocalManifests.
func (ms *FsManifests) ComponentInstalled(component, version string) (bool, error) {
	return ms.profile.VersionIsInstalled(component, version)
}

// InstallComponent implements LocalManifests.
func (ms *FsManifests) InstallComponent(reader io.Reader, targetDir, component, version, filename string, noExpand bool) error {
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

// TargetRootDir implements LocalManifests.
func (ms *FsManifests) TargetRootDir() string {
	return ms.profile.Root()
}

// Name implements LocalManifests.
func (ms *FsManifests) Name() string {
	return ms.profile.Name()
}

// copy from profile

// Path returns a full path which is related to profile root directory
func (ms *FsManifests) ProfilePath(relpath ...string) string {
	return ms.profile.Path()
}

// Root returns the root path of the `tiup`
func (ms *FsManifests) ProfileRoot() string {
	return ms.profile.Root()
}
func (ms *FsManifests) ProfileName() string {
	return ms.profile.Name()
}

// GetComponentInstalledVersion return the installed version of component.
func (ms *FsManifests) GetComponentInstalledVersion(component string, ver utils.Version) (utils.Version, error) {
	return ms.profile.GetComponentInstalledVersion(component, ver)
}

// ComponentInstalledPath returns the path where the component installed
func (ms *FsManifests) ComponentInstalledPath(component string, version utils.Version) (string, error) {
	return ms.profile.ComponentInstalledPath(component, version)
}

// InstalledComponents returns the installed components
func (ms *FsManifests) InstalledComponents() ([]string, error) {
	return ms.profile.InstalledComponents()
}

// InstalledVersions returns the installed versions of specific component
func (ms *FsManifests) InstalledVersions(component string) ([]string, error) {
	return ms.profile.InstalledVersions(component)
}

// VersionIsInstalled returns true if exactly version of component is installed.
func (ms *FsManifests) VersionIsInstalled(component, version string) (bool, error) {
	return ms.profile.VersionIsInstalled(component, version)
}

// ResetMirror reset root.json and cleanup manifests directory
func (ms *FsManifests) ResetMirror(addr, root string) error {
	return ms.profile.ResetMirror(addr, root)
}

// ReadMetaFile reads a Process object from dirName/MetaFilename. Returns (nil, nil) if a metafile does not exist.
func (ms *FsManifests) ReadMetaFile(dirName string) (*localdata.Process, error) {
	return ms.profile.ReadMetaFile(dirName)
}

// MockManifests is a LocalManifests implementation for testing.
type MockManifests struct {
	Manifests map[string]*Manifest
	Saved     []string
	Installed map[string]MockInstalled
	Ks        *KeyStore
}

// MockInstalled is used by MockManifests to remember what was installed for a component.
type MockInstalled struct {
	Version    string
	Contents   string
	BinaryPath string
}

// NewMockManifests creates an empty MockManifests.
func NewMockManifests() *MockManifests {
	return &MockManifests{
		Manifests: map[string]*Manifest{},
		Saved:     []string{},
		Installed: map[string]MockInstalled{},
		Ks:        NewKeyStore(),
	}
}

// SaveManifest implements LocalManifests.
func (ms *MockManifests) SaveManifest(manifest *Manifest, filename string) error {
	ms.Saved = append(ms.Saved, filename)
	ms.Manifests[filename] = manifest
	return loadKeys(manifest.Signed, ms.Ks)
}

// SaveComponentManifest implements LocalManifests.
func (ms *MockManifests) SaveComponentManifest(manifest *Manifest, filename string) error {
	ms.Saved = append(ms.Saved, filename)
	ms.Manifests[filename] = manifest
	return nil
}

// LoadManifest implements LocalManifests.
func (ms *MockManifests) LoadManifest(role ValidManifest) (*Manifest, bool, error) {
	manifest, ok := ms.Manifests[role.Filename()]
	if !ok {
		return nil, false, nil
	}

	switch role.Filename() {
	case ManifestFilenameRoot:
		ptr := role.(*Root)
		*ptr = *manifest.Signed.(*Root)
	case ManifestFilenameIndex:
		ptr := role.(*Index)
		*ptr = *manifest.Signed.(*Index)
	case ManifestFilenameSnapshot:
		ptr := role.(*Snapshot)
		*ptr = *manifest.Signed.(*Snapshot)
	case ManifestFilenameTimestamp:
		ptr := role.(*Timestamp)
		*ptr = *manifest.Signed.(*Timestamp)
	default:
		return nil, true, fmt.Errorf("unknown manifest type: %s", role.Filename())
	}

	err := loadKeys(role, ms.Ks)
	if err != nil {
		return nil, false, err
	}
	return manifest, true, nil
}

// LoadComponentManifest implements LocalManifests.
func (ms *MockManifests) LoadComponentManifest(item *ComponentItem, filename string) (*Component, error) {
	manifest, ok := ms.Manifests[filename]
	if !ok {
		return nil, nil
	}
	comp, ok := manifest.Signed.(*Component)
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
	_, err := io.Copy(&buf, reader)
	if err != nil {
		return err
	}
	ms.Installed[component] = MockInstalled{
		Version:    version,
		Contents:   buf.String(),
		BinaryPath: filepath.Join(targetDir, filename),
	}
	return nil
}

// KeyStore implements LocalManifests.
func (ms *MockManifests) KeyStore() *KeyStore {
	return ms.Ks
}

// TargetRootDir implements LocalManifests.
func (ms *MockManifests) TargetRootDir() string {
	return "/tmp/mock"
}

// Name implements LocalManifests.
func (ms *MockManifests) Name() string {
	return "mock"
}

// ManifestVersion implements LocalManifests.
func (ms *MockManifests) ManifestVersion(filename string) uint {
	manifest, ok := ms.Manifests[filename]
	if ok {
		return manifest.Signed.Base().Version
	}
	return 0
}

// copy from profile

// Path returns a full path which is related to profile root directory
func (ms *MockManifests) ProfilePath(relpath ...string) string {
	return "mock"
}

// Root returns the root path of the `tiup`
func (ms *MockManifests) ProfileRoot() string {
	return "mock"
}
func (ms *MockManifests) ProfileName() string {
	return "mock"
}

// GetComponentInstalledVersion return the installed version of component.
func (ms *MockManifests) GetComponentInstalledVersion(component string, ver utils.Version) (utils.Version, error) {
	return utils.Version(ms.Installed[component].Version), nil
}

// ComponentInstalledPath returns the path where the component installed
func (ms *MockManifests) ComponentInstalledPath(component string, version utils.Version) (string, error) {
	return "mock", nil
}

// InstalledComponents returns the installed components
func (ms *MockManifests) InstalledComponents() ([]string, error) {
	installed := []string{}

	for c, _ := range ms.Installed {
		installed = append(installed, c)
	}

	return installed, nil
}

// InstalledVersions returns the installed versions of specific component
func (ms *MockManifests) InstalledVersions(component string) ([]string, error) {

	return []string{ms.Installed[component].Version}, nil
}

// VersionIsInstalled returns true if exactly version of component is installed.
func (ms *MockManifests) VersionIsInstalled(component, version string) (bool, error) {
	return ms.Installed[component].Version == version, nil
}

// ResetMirror reset root.json and cleanup manifests directory
func (ms *MockManifests) ResetMirror(addr, root string) error {
	return nil
}

// ReadMetaFile reads a Process object from dirName/MetaFilename. Returns (nil, nil) if a metafile does not exist.
func (ms *MockManifests) ReadMetaFile(dirName string) (*localdata.Process, error) {
	return nil, nil
}
