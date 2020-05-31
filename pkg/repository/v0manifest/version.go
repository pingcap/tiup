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

package v0manifest

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

type (
	// Version represents a version string, like: v3.1.2
	Version string

	// VersionInfo represents the version information of component
	VersionInfo struct {
		Version   Version  `json:"version"`
		Date      string   `json:"date"`
		Entry     string   `json:"entry"`
		Platforms []string `json:"platforms"`
	}

	// VersionManifest represents the all versions information of specific component
	VersionManifest struct {
		Description string        `json:"description"`
		Modified    string        `json:"modified"`
		Nightly     *VersionInfo  `json:"nightly"`
		Versions    []VersionInfo `json:"versions"`
	}
)

// IsValid checks whether is the version string valid
func (v Version) IsValid() bool {
	return v != "" && semver.IsValid(string(v))
}

// IsEmpty returns true if the `Version` is a empty string
func (v Version) IsEmpty() bool {
	return v == ""
}

// IsNightly returns true if the version is nightly
func (v Version) IsNightly() bool {
	return strings.Contains(string(v), "nightly")
}

// String implements the fmt.Stringer interface
func (v Version) String() string {
	return string(v)
}

// Sort sorts all versions
func (manifest *VersionManifest) Sort() {
	sort.Slice(manifest.Versions, func(i, j int) bool {
		lhs := manifest.Versions[i].Version.String()
		rhs := manifest.Versions[j].Version.String()
		return semver.Compare(lhs, rhs) < 0
	})
}

// LatestVersion returns the latest stable version
func (manifest *VersionManifest) LatestVersion() Version {
	if len(manifest.Versions) > 0 {
		return manifest.Versions[len(manifest.Versions)-1].Version
	}
	return ""
}

// ContainsVersion returns if the versions contain the specific version
func (manifest *VersionManifest) ContainsVersion(version Version) bool {
	if version.IsNightly() && manifest.Nightly != nil {
		return true
	}
	for _, v := range manifest.Versions {
		if v.Version == version {
			return true
		}
	}
	return false
}

// FindVersion returns the specific version info
func (manifest *VersionManifest) FindVersion(version Version) (VersionInfo, bool) {
	if version.IsNightly() {
		if manifest.Nightly != nil {
			return *manifest.Nightly, true
		}
		return VersionInfo{}, false
	}
	for _, v := range manifest.Versions {
		if v.Version == version {
			return v, true
		}
	}
	return VersionInfo{}, false
}

// IterVersion iterates all versions
func (manifest *VersionManifest) IterVersion(fn func(versionInfo VersionInfo) error) error {
	if manifest.Nightly != nil {
		if err := fn(*manifest.Nightly); err != nil {
			return err
		}
	}
	for _, v := range manifest.Versions {
		if err := fn(v); err != nil {
			return err
		}
	}
	return nil
}

// IsSupport returns true if the specific version can be run in specified OS and architecture
func (c *VersionInfo) IsSupport(goos, goarch string) bool {
	s := fmt.Sprintf("%s/%s", goos, goarch)
	for _, p := range c.Platforms {
		if p == s {
			return true
		}
	}
	return false
}
