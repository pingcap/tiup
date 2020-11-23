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
	"testing"

	. "github.com/pingcap/check"
	pkgver "github.com/pingcap/tiup/pkg/repository/version"
)

func TestVersions(t *testing.T) {
	var c *C
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

	vm.Sort()
	c.Assert(vm.Versions[1].Version, Equals, pkgver.Version("v0.0.2"))
	c.Assert(vm.LatestVersion(), Equals, pkgver.Version("v0.0.3"))
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
