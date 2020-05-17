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
	. "github.com/pingcap/check"
)

type manifestSuite struct{}

var _ = Suite(&manifestSuite{})

func (s *manifestSuite) TestVersions(c *C) {
	vm := VersionManifest{
		Description: "test",
		Modified:    "test1",
		Nightly: &VersionInfo{
			Version: "nightly",
			Date:    "nightly-date",
			Entry:   "nightly-test",
			Platforms: []string{
				"linux/amd64", "darwin/amd64",
			},
		},
		Versions: []VersionInfo{
			{
				Version: "v0.0.1",
				Date:    "data1",
				Entry:   "entry",
				Platforms: []string{
					"linux/amd64", "darwin/amd64",
				},
			},
			{
				Version: "v0.0.3",
				Date:    "date3",
				Entry:   "entry",
				Platforms: []string{
					"linux/amd64", "darwin/amd64",
				},
			},
			{
				Version: "v0.0.2",
				Date:    "date2",
				Entry:   "entry",
				Platforms: []string{
					"linux/amd64", "darwin/amd64",
				},
			},
		},
	}

	c.Assert(Version("").IsValid(), IsFalse)
	c.Assert(Version("v3.0.").IsValid(), IsFalse)
	c.Assert(Version("").IsEmpty(), IsTrue)
	c.Assert(Version("").IsNightly(), IsFalse)
	c.Assert(Version("nightly").IsNightly(), IsTrue)

	c.Assert(Version("v3.0.0").String(), Equals, "v3.0.0")

	vm.Sort()
	c.Assert(vm.Versions[1].Version, Equals, Version("v0.0.2"))
	c.Assert(vm.LatestVersion(), Equals, Version("v0.0.3"))
	c.Assert(vm.ContainsVersion("v0.0.3"), IsTrue)
	c.Assert(vm.ContainsVersion("v0.0.4"), IsFalse)

	_, found := vm.FindVersion("v0.0.4")
	c.Assert(found, IsFalse)

	vi, found := vm.FindVersion("v0.0.3")
	c.Assert(found, IsTrue)
	c.Assert(vi.Version, DeepEquals, vm.LatestVersion())

	vi, found = vm.FindVersion("nightly")
	c.Assert(found, IsTrue)
	c.Assert(vi, DeepEquals, *vm.Nightly)

	c.Assert(vi.IsSupport("darwin", "amd64"), IsTrue)
	c.Assert(vi.IsSupport("darwin", "arm"), IsFalse)
}
