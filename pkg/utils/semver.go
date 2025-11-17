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
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"

	"slices"

	"github.com/pingcap/errors"
)

// NightlyVersionAlias represents latest build of master branch.
const NightlyVersionAlias = "nightly"

// LatestVersionAlias represents the latest build (excluding nightly versions).
const LatestVersionAlias = "latest"

// FTSVersionAlias represents the version alias for TiDB FTS feature branch.
const FTSVersionAlias = "v9.0.0-feature.fts"

// FmtVer converts a version string to SemVer format, if the string is not a valid
// SemVer and fails to parse and convert it, an error is raised.
func FmtVer(ver string) (string, error) {
	v := ver

	// nightly version is an alias
	if strings.ToLower(v) == NightlyVersionAlias {
		return v, nil
	}

	// latest version is an alias
	if strings.ToLower(v) == LatestVersionAlias {
		return v, nil
	}

	if !strings.HasPrefix(ver, "v") {
		v = fmt.Sprintf("v%s", ver)
	}
	if !semver.IsValid(v) {
		return v, fmt.Errorf("version %s is not a valid SemVer string", ver)
	}
	return v, nil
}

type (
	// Version represents a version string, like: v3.1.2
	Version string
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
	return strings.Contains(string(v), NightlyVersionAlias)
}

// String implements the fmt.Stringer interface
func (v Version) String() string {
	return string(v)
}

type ver struct {
	Major, Minor, Patch int
	Prerelease          []string
}

func (v *ver) Compare(other *ver) int {
	if d := compareSegment(v.Major, other.Major); d != 0 {
		return d
	}
	if d := compareSegment(v.Minor, other.Minor); d != 0 {
		return d
	}
	if d := compareSegment(v.Patch, other.Patch); d != 0 {
		return d
	}

	return comparePrerelease(v.Prerelease, other.Prerelease)
}

// Constraint for semver
type Constraint struct {
	// [min, max)
	min ver // min ver
	max ver // max ver
}

var (
	constraintRegex = regexp.MustCompile(
		`^(?P<constraint>[~^])?v?(?P<major>0|[1-9]\d*)(?:\.(?P<minor>0|[1-9]\d*|[x*]))?(?:\.(?P<patch>0|[1-9]\d*|[x*]))?(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<build>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`,
	)

	versionRegexp = regexp.MustCompile(
		`^v?(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<build>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`,
	)
)

// NewConstraint creates a constraint to check whether a semver is valid.
// Only support ^ and ~ and x|X|*
func NewConstraint(raw string) (*Constraint, error) {
	result := MatchGroups(constraintRegex, strings.ToLower(strings.TrimSpace(raw)))
	if len(result) == 0 {
		return nil, errors.New("fail to parse version constraint")
	}
	c := &Constraint{}
	defer func() {
		if len(c.max.Prerelease) == 0 {
			c.max.Prerelease = []string{"0"}
		}
	}()

	c.min.Major = MustAtoI(result["major"])
	c.max.Major = c.min.Major

	if minor := result["minor"]; minor == "x" || minor == "*" {
		c.max.Major = c.min.Major + 1
		return c, nil
	} else if minor != "" {
		c.min.Minor = MustAtoI(result["minor"])
		c.max.Minor = c.min.Minor
	}

	if patch := result["patch"]; patch == "x" || patch == "*" {
		c.max.Minor = c.min.Minor + 1
		return c, nil
	} else if patch != "" {
		c.min.Patch = MustAtoI(result["patch"])
		c.max.Patch = c.min.Patch
	}

	if prerelease := result["prerelease"]; prerelease != "" {
		c.min.Prerelease = strings.Split(prerelease, ".")
	} else {
		c.min.Prerelease = []string{}
	}

	c.max.Prerelease = slices.Clone(c.min.Prerelease)

	if constraint := result["constraint"]; constraint == "~" {
		// ~x.y.z -> >=x.y.z <x.(y+1).0
		c.max.Minor++
		c.max.Patch = 0
	} else if constraint == "^" {
		if c.min.Major == 0 {
			if c.min.Minor == 0 {
				// ^0.0.z -> =0.0.z
				c.max.Patch++
			} else {
				// ^0.y.z -> >=0.y.z <0.(y+1).0
				c.max.Minor++
				c.max.Patch = 0
			}
		} else {
			// ^x.y.z -> >=x.y.z <(x+1).0.0
			c.max.Major++
			c.max.Minor = 0
			c.max.Patch = 0
		}
	} else if l := len(c.max.Prerelease); l > 0 {
		c.max.Prerelease[l-1] += " "
	} else {
		c.max.Patch++
	}

	return c, nil
}

// Check checks whether a version is satisfies the constraint
func (c *Constraint) Check(v string) bool {
	result := MatchGroups(versionRegexp, strings.ToLower(strings.TrimSpace(v)))
	if len(result) == 0 {
		return false
	}

	major := MustAtoI(result["major"])
	minor := MustAtoI(result["minor"])
	patch := MustAtoI(result["patch"])
	version := &ver{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: []string{},
	}
	if pre := result["prerelease"]; pre != "" {
		version.Prerelease = strings.Split(pre, ".")
	}
	return c.min.Compare(version) <= 0 && c.max.Compare(version) > 0
}

func compareSegment(v, o int) int {
	if v < o {
		return -1
	}
	if v > o {
		return 1
	}
	return 0
}

// 1, -1, 0 means A>B, A<B, A=B
func comparePrerelease(preA, preB []string) int {
	lenA := len(preA)
	lenB := len(preB)
	if lenA == 0 && lenB == 0 {
		return 0
	}
	if lenA == 0 {
		return 1
	}
	if lenB == 0 {
		return -1
	}
	for i, tempA := range preA {
		if i == lenB {
			// alpha < alpha.1
			return 1
		}
		if d := compareAlphaNum(tempA, preB[i]); d != 0 {
			return d
		}
	}
	if lenB > len(preA) {
		return -1
	}
	return 0
}

func compareAlphaNum(a, b string) int {
	if a == b {
		return 0
	}
	iA, errA := strconv.Atoi(a)
	iB, errB := strconv.Atoi(b)
	if errA != nil && errB != nil {
		if a > b {
			return 1
		}
		return -1
	}
	// Numeric identifiers always have lower precedence than non-numeric identifiers.
	if errA != nil {
		return 1
	}
	if errB != nil {
		return -1
	}
	if iA > iB {
		return 1
	}
	return -1
}
