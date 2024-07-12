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

package spec

import (
	"github.com/pingcap/check"
)

type utilSuite struct{}

var _ = check.Suite(&utilSuite{})

func (s utilSuite) TestAbs(c *check.C) {
	var path string
	path = Abs(" foo", "")
	c.Assert(path, check.Equals, "/home/foo")
	path = Abs("foo ", " ")
	c.Assert(path, check.Equals, "/home/foo")
	path = Abs("foo", "bar")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", " bar")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", "bar ")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", " bar ")
	c.Assert(path, check.Equals, "/home/foo/bar")
	path = Abs("foo", "/bar")
	c.Assert(path, check.Equals, "/bar")
	path = Abs("foo", " /bar")
	c.Assert(path, check.Equals, "/bar")
	path = Abs("foo", "/bar ")
	c.Assert(path, check.Equals, "/bar")
	path = Abs("foo", " /bar ")
	c.Assert(path, check.Equals, "/bar")
}

func (s *utilSuite) TestMultiDirAbs(c *check.C) {
	paths := MultiDirAbs("tidb", "")
	c.Assert(len(paths), check.Equals, 0)

	paths = MultiDirAbs("tidb", " ")
	c.Assert(len(paths), check.Equals, 0)

	paths = MultiDirAbs("tidb", "a ")
	c.Assert(len(paths), check.Equals, 1)
	c.Assert(paths[0], check.Equals, "/home/tidb/a")

	paths = MultiDirAbs("tidb", "a , /tmp/b")
	c.Assert(len(paths), check.Equals, 2)
	c.Assert(paths[0], check.Equals, "/home/tidb/a")
	c.Assert(paths[1], check.Equals, "/tmp/b")
}
