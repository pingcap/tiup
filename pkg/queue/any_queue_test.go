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

package queue

import (
	"reflect"

	"github.com/pingcap/check"
)

var _ = check.Suite(&queueTestSuite{})

type queueTestSuite struct{}

func (s *queueTestSuite) TestAnyQueue(c *check.C) {
	q := NewAnyQueue(reflect.DeepEqual)

	q.Put(true)
	q.Put(9527)

	c.Assert(q.slice[0], check.DeepEquals, true)
	c.Assert(q.slice[1], check.DeepEquals, 9527)

	q.Put(true)
	q.Put(9527)

	c.Assert(q.slice[2], check.DeepEquals, true)
	c.Assert(q.slice[3], check.DeepEquals, 9527)

	c.Assert(q.Get(true), check.DeepEquals, true)
	c.Assert(q.Get(true), check.DeepEquals, true)
	c.Assert(q.Get(true), check.DeepEquals, nil)

	c.Assert(q.Get(9527), check.DeepEquals, 9527)
	c.Assert(q.Get(9527), check.DeepEquals, 9527)
	c.Assert(q.Get(9527), check.DeepEquals, nil)
}
