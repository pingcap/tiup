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

package pkgver

import (
	"testing"

	. "github.com/pingcap/check"
)

func TestVersion(t *testing.T) {
	var c *C
	c.Assert(Version("").IsValid(), IsFalse)
	c.Assert(Version("v3.0.").IsValid(), IsFalse)
	c.Assert(Version("").IsEmpty(), IsTrue)
	c.Assert(Version("").IsNightly(), IsFalse)
	c.Assert(Version("nightly").IsNightly(), IsTrue)
	c.Assert(Version("v1.2.3").String(), Equals, "v1.2.3")
}
