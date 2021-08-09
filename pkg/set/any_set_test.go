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

package set

import (
	"reflect"

	"github.com/pingcap/check"
)

func (s *setTestSuite) TestAnySet(c *check.C) {
	set := NewAnySet(reflect.DeepEqual)
	set.Insert(true)
	set.Insert(9527)

	c.Assert(set.slice[0], check.DeepEquals, true)
	c.Assert(set.Slice()[0], check.DeepEquals, true)

	c.Assert(set.slice[1], check.DeepEquals, 9527)
	c.Assert(set.Slice()[1], check.DeepEquals, 9527)
}
