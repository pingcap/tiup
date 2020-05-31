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

package utils

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// FmtVer converts a version string to SemVer format, if the string is not a valid
// SemVer and fails to parse and convert it, an error is raised.
func FmtVer(ver string) (string, error) {
	v := ver
	if !strings.HasPrefix(ver, "v") {
		v = fmt.Sprintf("v%s", ver)
	}
	if !semver.IsValid(v) {
		return v, fmt.Errorf("version %s is not a valid SemVer string", ver)
	}
	return v, nil
}
