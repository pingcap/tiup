// Copyright 2021 PingCAP, Inc.
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

// This file only contains version related variables and consts, all
// type definitions and functions shall not be implemented here.
// This file is excluded from CI tests.

package version

var (
	// TiUPVerMajor is the major version of TiUP
	TiUPVerMajor = 1
	// TiUPVerMinor is the minor version of TiUP
	TiUPVerMinor = 9
	// TiUPVerPatch is the patch version of TiUP
	TiUPVerPatch = 4
	// TiUPVerName is an alternative name of the version
	TiUPVerName = "tiup"
	// GitHash is the current git commit hash
	GitHash = "Unknown"
	// GitRef is the current git reference name (branch or tag)
	GitRef = "Unknown"
)
