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
	"sort"

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
		Nightly   bool     `json:"nightly"`
		Platforms []string `json:"platforms"`
	}

	// VersionManifest represents the all versions information of specific component
	VersionManifest struct {
		Description string        `json:"description"`
		Modified    string        `json:"modified"`
		Versions    []VersionInfo `json:"versions"`
	}
)

// IsValid checks whether is the version string valid
func (v Version) IsValid() bool {
	return v != "" && semver.IsValid(string(v))
}

func (v Version) String() string {
	return string(v)
}

func (manifest *VersionManifest) sort() {
	sort.Slice(manifest.Versions, func(i, j int) bool {
		lhs := manifest.Versions[i].Version.String()
		rhs := manifest.Versions[j].Version.String()
		return semver.Compare(lhs, rhs) < 0
	})
}

// LatestStable returns the latest stable version
func (manifest *VersionManifest) LatestStable() Version {
	for i := len(manifest.Versions) - 1; i >= 0; i-- {
		if !manifest.Versions[i].Nightly {
			return manifest.Versions[i].Version
		}
	}
	return ""
}

// LatestNightly returns the latest nightly version
func (manifest *VersionManifest) LatestNightly() Version {
	for i := len(manifest.Versions) - 1; i >= 0; i-- {
		if manifest.Versions[i].Nightly {
			return manifest.Versions[i].Version
		}
	}
	return ""
}
