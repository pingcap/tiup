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
	// TiOpsVerMajor is the major version of TiOps
	TiOpsVerMajor = 0
	// TiOpsVerMinor is the minor version of TiOps
	TiOpsVerMinor = 3
	// TiOpsVerPatch is the patch version of TiOps
	TiOpsVerPatch = 0
	// GitHash is the current git commit hash
	GitHash = "Unknown"
	// GitBranch is the current git branch name
	GitBranch = "Unknown"
)

// TiOpsVersion is the semver of TiOps
type TiOpsVersion struct {
	major int
	minor int
	patch int
	name  string
}

// NewTiOpsVersion creates a TiOpsVersion object
func NewTiOpsVersion() *TiOpsVersion {
	return &TiOpsVersion{
		major: TiOpsVerMajor,
		minor: TiOpsVerMinor,
		patch: TiOpsVerPatch,
	}
}

// SemVer returns TiOpsVersion in semver format
func (v *TiOpsVersion) SemVer() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

// String converts TiOpsVersion to a string
func (v *TiOpsVersion) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

// TiOpsBuild is the info of building environment
type TiOpsBuild struct {
	GitHash   string `json:"gitHash"`
	GitBranch string `json:"gitBranch"`
	GoVersion string `json:"goVersion"`
}

// NewTiOpsBuildInfo creates a TiOpsBuild object
func NewTiOpsBuildInfo() *TiOpsBuild {
	return &TiOpsBuild{
		GitHash:   GitHash,
		GitBranch: GitBranch,
		GoVersion: runtime.Version(),
	}
}

// String converts TiOpsBuild to a string
func (v *TiOpsBuild) String() string {
	return fmt.Sprintf("%s %s(%s)", v.GoVersion, v.GitBranch, v.GitHash)
}
