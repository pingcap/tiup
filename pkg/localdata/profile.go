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
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sort"

	"github.com/pingcap-incubator/tiup/pkg/repository/v0manifest"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"golang.org/x/mod/semver"
)

// Profile represents the `tiup` profile
type Profile struct {
	root string
}

// NewProfile returns a new profile instance
func NewProfile(root string) *Profile {
	return &Profile{root: root}
}

// InitProfile creates a new profile using environment variables and defaults.
func InitProfile() *Profile {
	var profileDir string
	switch {
	case os.Getenv(EnvNameHome) != "":
		profileDir = os.Getenv(EnvNameHome)
	case DefaultTiupHome != "":
		profileDir = DefaultTiupHome
	default:
		u, err := user.Current()
		if err != nil {
			panic("cannot get current user information: " + err.Error())
		}
		profileDir = filepath.Join(u.HomeDir, ProfileDirName)
	}
	return NewProfile(profileDir)
}

// Path returns a full path which is related to profile root directory
func (p *Profile) Path(relpath ...string) string {
	return filepath.Join(append([]string{p.root}, relpath...)...)
}

// Root returns the root path of the `tiup`
func (p *Profile) Root() string {
	return p.root
}

// BinaryPath returns the binary path of component specific version
func (p *Profile) BinaryPath(component string, version v0manifest.Version) (string, error) {
	manifest := p.Versions(component)
	if manifest == nil {
		return "", errors.Errorf("component `%s` doesn't install", component)
	}
	var entry string
	if version.IsNightly() && manifest.Nightly != nil {
		entry = manifest.Nightly.Entry
	} else {
		for _, v := range manifest.Versions {
			if v.Version == version {
				entry = v.Entry
			}
		}
	}
	if entry == "" {
		return "", errors.Errorf("cannot found entry for %s:%s", component, version)
	}
	installPath, err := p.ComponentInstalledPath(component, version)
	if err != nil {
		return "", err
	}
	return filepath.Join(installPath, entry), nil
}

// ComponentInstalledPath returns the path where the component installed
func (p *Profile) ComponentInstalledPath(component string, version v0manifest.Version) (string, error) {
	versions, err := p.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	// Use the latest version if user doesn't specify a specific version
	// report an error if the specific component doesn't be installed

	// Check whether the specific version exist in local
	if version.IsEmpty() && len(versions) > 0 {
		sort.Slice(versions, func(i, j int) bool {
			return semver.Compare(versions[i], versions[j]) < 0
		})
		version = v0manifest.Version(versions[len(versions)-1])
	} else if version.IsEmpty() {
		return "", fmt.Errorf("component not installed, please try `tiup install %s` to install it", component)
	}
	return filepath.Join(p.Path(ComponentParentDir), component, version.String()), nil
}

// SaveTo saves file to the profile directory, path is relative to the
// profile directory of current user
func (p *Profile) SaveTo(path string, data []byte, perm os.FileMode) error {
	fullPath := filepath.Join(p.root, path)
	// create sub directory if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return errors.Trace(err)
	}
	return ioutil.WriteFile(fullPath, data, perm)
}

// WriteJSON writes struct to a file (in the profile directory) in JSON format
func (p *Profile) WriteJSON(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.Trace(err)
	}
	return p.SaveTo(path, jsonData, 0644)
}

// ReadJSON read file and unmarshal to target `data`
func (p *Profile) ReadJSON(path string, data interface{}) error {
	fullPath := filepath.Join(p.root, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(data)
}

func (p *Profile) versionFileName(component string) string {
	return fmt.Sprintf("manifest/tiup-component-%s.index", component)
}

func (p *Profile) v0ManifestFileName() string {
	return "manifest/tiup-manifest.index"
}

func (p *Profile) isNotExist(path string) bool {
	return utils.IsNotExist(p.Path(path))
}

// Manifest returns the components manifest
func (p *Profile) Manifest() *v0manifest.ComponentManifest {
	if p.isNotExist(p.v0ManifestFileName()) {
		return nil
	}

	var manifest v0manifest.ComponentManifest
	if err := p.ReadJSON(p.v0ManifestFileName(), &manifest); err != nil {
		// The manifest was marshaled and stored by `tiup`, it should
		// be a valid JSON file
		log.Fatal(err)
	}

	return &manifest
}

// SaveManifest saves the latest components manifest to local profile
func (p *Profile) SaveManifest(manifest *v0manifest.ComponentManifest) error {
	return p.WriteJSON(p.v0ManifestFileName(), manifest)
}

// Versions returns the version manifest of specific component
func (p *Profile) Versions(component string) *v0manifest.VersionManifest {
	file := p.versionFileName(component)
	if p.isNotExist(file) {
		return nil
	}

	var manifest v0manifest.VersionManifest
	if err := p.ReadJSON(file, &manifest); err != nil {
		// The manifest was marshaled and stored by `tiup`, it should
		// be a valid JSON file
		log.Fatal(err)
	}

	return &manifest
}

// SaveVersions saves the latest version manifest to local profile of specific component
func (p *Profile) SaveVersions(component string, manifest *v0manifest.VersionManifest) error {
	return p.WriteJSON(p.versionFileName(component), manifest)
}

// InstalledComponents returns the installed components
func (p *Profile) InstalledComponents() ([]string, error) {
	compDir := filepath.Join(p.root, ComponentParentDir)
	fileInfos, err := ioutil.ReadDir(compDir)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	var components []string
	for _, fi := range fileInfos {
		if !fi.IsDir() {
			continue
		}
		components = append(components, fi.Name())
	}
	sort.Strings(components)
	return components, nil
}

// InstalledVersions returns the installed versions of specific component
func (p *Profile) InstalledVersions(component string) ([]string, error) {
	path := filepath.Join(p.root, ComponentParentDir, component)
	if utils.IsNotExist(path) {
		return nil, nil
	}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var versions []string
	for _, fi := range fileInfos {
		if !fi.IsDir() {
			continue
		}
		sub, err := ioutil.ReadDir(filepath.Join(path, fi.Name()))
		if err != nil || len(sub) < 1 {
			continue
		}
		versions = append(versions, fi.Name())
	}
	return versions, nil
}

// SelectInstalledVersion selects the installed versions and the latest release version
// will be chosen if there is an empty version
func (p *Profile) SelectInstalledVersion(component string, version v0manifest.Version) (v0manifest.Version, error) {
	installed, err := p.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	errInstallFirst := fmt.Errorf("use `tiup install %[1]s` to install `%[1]s` first", component)
	if len(installed) < 1 {
		return "", errInstallFirst
	}

	if version.IsEmpty() {
		sort.Slice(installed, func(i, j int) bool {
			return semver.Compare(installed[i], installed[j]) < 0
		})
		version = v0manifest.Version(installed[len(installed)-1])
	}
	found := false
	for _, v := range installed {
		if v0manifest.Version(v) == version {
			found = true
			break
		}
	}
	if !found {
		return "", errInstallFirst
	}
	return version, nil
}
