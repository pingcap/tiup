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
	"errors"
	"fmt"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

// Manifest representation for ser/de.
type Manifest struct {
	// Signatures value
	Signatures []Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed ValidManifest `json:"signed"`
}

// RawManifest representation for ser/de.
type RawManifest struct {
	// Signatures value
	Signatures []Signature `json:"signatures"`
	// Signed value; raw json message
	Signed json.RawMessage `json:"signed"`
}

// Signature represents a signature for a manifest
type Signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

// SignedBase represents parts of a manifest's signed value which are shared by all manifests.
type SignedBase struct {
	Ty          string `json:"_type"`
	SpecVersion string `json:"spec_version"`
	Expires     string `json:"expires"`
	// 0 => no version specified
	Version uint `json:"version"`
}

// SetExpiresAt set manifest expires at the specified time.
func (s *SignedBase) SetExpiresAt(t time.Time) {
	s.Expires = t.Format(time.RFC3339)
}

// ValidManifest is a manifest which includes SignedBase and can be validated.
type ValidManifest interface {
	isValid() error
	// Base returns this manifest's SignedBase which is values common to all manifests.
	Base() *SignedBase
	// Filename returns the unversioned name that the manifest should be saved as based on its Go type.
	Filename() string
}

// Root manifest.
type Root struct {
	SignedBase
	Roles map[string]*Role `json:"roles"`
}

// Role object.
type Role struct {
	URL       string              `json:"url"`
	Keys      map[string]*KeyInfo `json:"keys"`
	Threshold uint                `json:"threshold"`
}

// KeyInfo is the manifest structure of a single key
type KeyInfo struct {
	Type   string            `json:"keytype"`
	Value  map[string]string `json:"keyval"`
	Scheme string            `json:"scheme"`
}

// Index manifest.
type Index struct {
	SignedBase
	Owners            map[string]Owner         `json:"owners"`
	Components        map[string]ComponentItem `json:"components"`
	DefaultComponents []string                 `json:"default_components"`
}

// Owner object.
type Owner struct {
	Name      string              `json:"name"`
	Keys      map[string]*KeyInfo `json:"keys"`
	Threshold int                 `json:"threshold"`
}

// VersionItem is the manifest structure of a version of a component
type VersionItem struct {
	URL          string            `json:"url"`
	Yanked       bool              `json:"yanked"`
	Entry        string            `json:"entry"`
	Released     string            `json:"released"`
	Dependencies map[string]string `json:"dependencies"`

	FileHash
}

// Component manifest.
type Component struct {
	SignedBase
	ID          string `json:"id"`
	Description string `json:"description"`
	Nightly     string `json:"nightly"` // version of the latest daily build
	// platform -> version -> VersionItem
	Platforms map[string]map[string]VersionItem `json:"platforms"`
}

// ComponentItem object
type ComponentItem struct {
	Yanked     bool   `json:"yanked"`
	Owner      string `json:"owner"`
	URL        string `json:"url"`
	Standalone bool   `json:"standalone"`
	Hidden     bool   `json:"hidden"`
}

// Snapshot manifest.
type Snapshot struct {
	SignedBase
	Meta map[string]FileVersion `json:"meta"`
}

// Timestamp manifest.
type Timestamp struct {
	SignedBase
	Meta map[string]FileHash `json:"meta"`
}

// FileHash is the hashes and length of a file.
type FileHash struct {
	Hashes map[string]string `json:"hashes"`
	Length uint              `json:"length"`
}

// FileVersion is just a version number.
type FileVersion struct {
	Version uint `json:"version"`
	Length  uint `json:"length"`
}

// Boilerplate implementations of ValidManifest.

// Base implements ValidManifest
func (manifest *Root) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Index) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Component) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Snapshot) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Timestamp) Base() *SignedBase {
	return &manifest.SignedBase
}

// Filename implements ValidManifest
func (manifest *Root) Filename() string {
	return ManifestFilenameRoot
}

// Filename implements ValidManifest
func (manifest *Index) Filename() string {
	return ManifestFilenameIndex
}

// ComponentList returns non-yanked components
func (manifest *Index) ComponentList() map[string]ComponentItem {
	components := make(map[string]ComponentItem)
	for n, c := range manifest.Components {
		if c.Yanked {
			continue
		}
		components[n] = c
	}
	return components
}

// ComponentListWithYanked return all components
func (manifest *Index) ComponentListWithYanked() map[string]ComponentItem {
	return manifest.Components
}

// Filename implements ValidManifest
func (manifest *Component) Filename() string {
	return manifest.ID + ".json"
}

// Filename implements ValidManifest
func (manifest *Snapshot) Filename() string {
	return ManifestFilenameSnapshot
}

// Filename implements ValidManifest
func (manifest *Timestamp) Filename() string {
	return ManifestFilenameTimestamp
}

// HasNightly return true if the component has nightly version.
func (manifest *Component) HasNightly(platform string) bool {
	if manifest.Nightly == "" {
		return false
	}

	v, ok := manifest.Platforms[platform][manifest.Nightly]
	if !ok {
		return false
	}

	return !v.Yanked
}

// VersionList return all versions exclude yanked versions
func (manifest *Component) VersionList(platform string) map[string]VersionItem {
	versions := make(map[string]VersionItem)

	vs := manifest.VersionListWithYanked(platform)
	if vs == nil {
		return nil
	}

	for v, vi := range vs {
		if vi.Yanked {
			continue
		}
		versions[v] = vi
	}

	return versions
}

// LatestVersion return the latest version exclude yanked versions
func (manifest *Component) LatestVersion(platform string) string {
	versions := manifest.VersionList(platform)
	if versions == nil {
		return ""
	}

	var latest string
	for v := range versions {
		if utils.Version(v).IsNightly() {
			continue
		}
		if latest == "" || semver.Compare(v, latest) > 0 {
			latest = v
		}
	}
	return latest
}

// VersionListWithYanked return all versions include yanked versions
func (manifest *Component) VersionListWithYanked(platform string) map[string]VersionItem {
	if manifest == nil {
		return nil
	}
	vs, ok := manifest.Platforms[platform]
	if !ok {
		vs, ok = manifest.Platforms[AnyPlatform]
		if !ok {
			return nil
		}
	}

	return vs
}

// ErrLoadManifest is an empty object of LoadManifestError, useful for type check
var ErrLoadManifest = &LoadManifestError{}

// LoadManifestError is the error type used when loading manifest failes
type LoadManifestError struct {
	manifest string // manifest name
	err      error  // wrapped error
}

// Error implements the error interface
func (e *LoadManifestError) Error() string {
	return fmt.Sprintf(
		"error loading manifest %s: %s",
		e.manifest, e.err,
	)
}

// Unwrap implements the error interface
func (e *LoadManifestError) Unwrap() error { return e.err }

// Is implements the error interface
func (e *LoadManifestError) Is(target error) bool {
	t, ok := target.(*LoadManifestError)
	if !ok {
		return false
	}

	return (e.manifest == t.manifest || t.manifest == "") &&
		(errors.Is(e.err, t.err) || t.err == nil)
}
