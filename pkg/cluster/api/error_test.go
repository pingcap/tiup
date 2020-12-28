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

package api

import (
	"errors"
	"testing"

	"github.com/pingcap/check"
)

func TestNoStoreErrIs(t *testing.T) {
	var c *check.C
	err0 := &NoStoreErr{
		addr: "1.2.3.4",
	}
	// identical errors are equal
	c.Assert(errors.Is(err0, err0), check.IsTrue)
	c.Assert(errors.Is(ErrNoStore, ErrNoStore), check.IsTrue)
	c.Assert(errors.Is(ErrNoStore, &NoStoreErr{}), check.IsTrue)
	c.Assert(errors.Is(&NoStoreErr{}, ErrNoStore), check.IsTrue)
	// not equal for different error types
	c.Assert(errors.Is(err0, errors.New("")), check.IsFalse)
	// default Value matches any error
	c.Assert(errors.Is(err0, ErrNoStore), check.IsTrue)
	// error with values are not matching default ones
	c.Assert(errors.Is(ErrNoStore, err0), check.IsFalse)

	err1 := &NoStoreErr{
		addr: "2.3.4.5",
	}
	c.Assert(errors.Is(err1, ErrNoStore), check.IsTrue)
	// errors with different values are not equal
	c.Assert(errors.Is(err0, err1), check.IsFalse)
	c.Assert(errors.Is(err1, err0), check.IsFalse)
}
