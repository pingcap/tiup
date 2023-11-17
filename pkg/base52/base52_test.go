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

package base52

import (
	"testing"

	. "github.com/pingcap/check"
)

func Test(t *testing.T) { TestingT(t) }

type base52Suite struct{}

var _ = Suite(&base52Suite{})

func (s *base52Suite) TestEncode(c *C) {
	c.Assert(Encode(1000000000), Equals, "2TPzw7")
}

func (s *base52Suite) TestDecode(c *C) {
	decoded, err := Decode("2TPzw7")
	c.Assert(decoded, Equals, int64(1000000000))
	c.Assert(err, IsNil)

	decoded, err = Decode("../../etc/passwd")
	c.Assert(decoded, Equals, int64(0))
	c.Assert(err, NotNil)
}
