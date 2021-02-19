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

package version

import (
	"fmt"
	"runtime"
)

var (
	// TiUPVerMajor is the major version of TiUP
	TiUPVerMajor = 1
	// TiUPVerMinor is the minor version of TiUP
	TiUPVerMinor = 3
	// TiUPVerPatch is the patch version of TiUP
	TiUPVerPatch = 2
	// TiUPVerName is alternative name of the version
	TiUPVerName = "tiup"
	// GitHash is the current git commit hash
	GitHash = "Unknown"
	// GitRef is the current git reference name (branch or tag)
	GitRef = "Unknown"
)

// NightlyVersion represents latest build of master branch.
const NightlyVersion = "nightly"

// TiUPVersion is the semver of TiUP
type TiUPVersion struct {
	major int
	minor int
	patch int
	name  string
}

// NewTiUPVersion creates a TiUPVersion object
func NewTiUPVersion() *TiUPVersion {
	return &TiUPVersion{
		major: TiUPVerMajor,
		minor: TiUPVerMinor,
		patch: TiUPVerPatch,
		name:  TiUPVerName,
	}
}

// Name returns the alternave name of TiUPVersion
func (v *TiUPVersion) Name() string {
	return v.name
}

// SemVer returns TiUPVersion in semver format
func (v *TiUPVersion) SemVer() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

// String converts TiUPVersion to a string
func (v *TiUPVersion) String() string {
	return fmt.Sprintf("v%d.%d.%d %s\n%s", v.major, v.minor, v.patch, v.name, NewTiUPBuildInfo())
}

// TiUPBuild is the info of building environment
type TiUPBuild struct {
	GitHash   string `json:"gitHash"`
	GitRef    string `json:"gitRef"`
	GoVersion string `json:"goVersion"`
}

// NewTiUPBuildInfo creates a TiUPBuild object
func NewTiUPBuildInfo() *TiUPBuild {
	return &TiUPBuild{
		GitHash:   GitHash,
		GitRef:    GitRef,
		GoVersion: runtime.Version(),
	}
}

// String converts TiUPBuild to a string
func (v *TiUPBuild) String() string {
	return fmt.Sprintf("Go Version: %s\nGit Ref: %s\nGitHash: %s", v.GoVersion, v.GitRef, v.GitHash)
}
