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
	TiOpsVerMajor = 1
	// TiOpsVerMinor is the minor version of TiOps
	TiOpsVerMinor = 0
	// TiOpsVerPatch is the patch version of TiOps
	TiOpsVerPatch = 0
	// GitHash is the current git commit hash
	GitHash = "Unknown"
	// GitBranch is the current git branch name
	GitBranch = "Unknown"
)

// TiOpsVersion is the semver of TiOps
type TiOpsVersion struct {
	major     int
	minor     int
	patch     int
	gitBranch string
	gitHash   string
	goVersion string
}

// NewTiOpsVersion creates a TiOpsVersion object
func NewTiOpsVersion() *TiOpsVersion {
	return &TiOpsVersion{
		major:     TiOpsVerMajor,
		minor:     TiOpsVerMinor,
		patch:     TiOpsVerPatch,
		gitBranch: GitBranch,
		gitHash:   GitHash,
		goVersion: runtime.Version(),
	}
}

// SemVer returns TiOpsVersion in semver format
func (v *TiOpsVersion) SemVer() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

// String converts TiOpsVersion to a string
func (v *TiOpsVersion) String() string {
	return v.SemVer()
}

// FullInfo returns full version and build info
func (v *TiOpsVersion) FullInfo() string {
	return fmt.Sprintf("%s (%s/%s) %s",
		v.String(),
		v.gitBranch,
		v.gitHash,
		v.goVersion)
}
