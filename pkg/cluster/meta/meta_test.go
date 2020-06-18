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

package meta

import (
	"errors"

	"github.com/pingcap/check"
)

type metaSuite struct {
}

var _ = check.Suite(&metaSuite{})

func (s *metaSuite) TestValidateErrIs(c *check.C) {
	err0 := &validateErr{
		ty:     "dummy",
		target: "test",
		one:    "one",
		two:    "two",
		value:  1,
	}
	// identical errors are equal
	c.Assert(errors.Is(err0, err0), check.IsTrue)
	c.Assert(errors.Is(ValidateErr, ValidateErr), check.IsTrue)
	c.Assert(errors.Is(ValidateErr, &validateErr{}), check.IsTrue)
	c.Assert(errors.Is(&validateErr{}, ValidateErr), check.IsTrue)
	// not equal for different error types
	c.Assert(errors.Is(err0, errors.New("")), check.IsFalse)
	// default value matches any error
	c.Assert(errors.Is(err0, ValidateErr), check.IsTrue)
	// error with values are not matching default ones
	c.Assert(errors.Is(ValidateErr, err0), check.IsFalse)

	err1 := &validateErr{
		ty:     errTypeConflict,
		target: "test",
		one:    "one",
		two:    "two",
		value:  2,
	}
	c.Assert(errors.Is(err1, ValidateErr), check.IsTrue)
	// errors with different values are not equal
	c.Assert(errors.Is(err0, err1), check.IsFalse)
	c.Assert(errors.Is(err1, err0), check.IsFalse)

	// errors with different types are not equal
	err0.value = 2
	c.Assert(errors.Is(err0, err1), check.IsFalse)
	c.Assert(errors.Is(err1, err0), check.IsFalse)

	err2 := &validateErr{
		ty:     errTypeMismatch,
		target: "test",
		one:    "one",
		two:    "two",
		value: map[string]interface{}{
			"key1": 1,
			"key2": "2",
		},
	}
	c.Assert(errors.Is(err2, ValidateErr), check.IsTrue)
	c.Assert(errors.Is(err1, err2), check.IsFalse)
	c.Assert(errors.Is(err2, err1), check.IsFalse)

	err3 := &validateErr{
		ty:     errTypeMismatch,
		target: "test",
		one:    "one",
		two:    "two",
		value:  []float64{1.0, 2.0},
	}
	c.Assert(errors.Is(err3, ValidateErr), check.IsTrue)
	// different values are not equal
	c.Assert(errors.Is(err2, err3), check.IsFalse)
	c.Assert(errors.Is(err3, err2), check.IsFalse)

	err4 := &validateErr{
		ty:     errTypeMismatch,
		target: "test",
		one:    "one",
		two:    "two",
		value:  nil,
	}
	c.Assert(errors.Is(err4, ValidateErr), check.IsTrue)
	// nil value matches any error if other fields are with the same values
	c.Assert(errors.Is(err3, err4), check.IsTrue)
	c.Assert(errors.Is(err4, err3), check.IsFalse)

	err4.value = 0
	c.Assert(errors.Is(err4, ValidateErr), check.IsTrue)
	c.Assert(errors.Is(err3, err4), check.IsFalse)
	c.Assert(errors.Is(err4, err3), check.IsFalse)
}
