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
	"testing"

	"github.com/pingcap/check"
)

func TestValidateErrIs(t *testing.T) {
	var c *check.C
	err0 := &ValidateErr{
		Type:   "dummy",
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  1,
	}
	// identical errors are equal
	c.Assert(errors.Is(err0, err0), check.IsTrue)
	c.Assert(errors.Is(ErrValidate, ErrValidate), check.IsTrue)
	c.Assert(errors.Is(ErrValidate, &ValidateErr{}), check.IsTrue)
	c.Assert(errors.Is(&ValidateErr{}, ErrValidate), check.IsTrue)
	// not equal for different error types
	c.Assert(errors.Is(err0, errors.New("")), check.IsFalse)
	// default Value matches any error
	c.Assert(errors.Is(err0, ErrValidate), check.IsTrue)
	// error with values are not matching default ones
	c.Assert(errors.Is(ErrValidate, err0), check.IsFalse)

	err1 := &ValidateErr{
		Type:   TypeConflict,
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  2,
	}
	c.Assert(errors.Is(err1, ErrValidate), check.IsTrue)
	// errors with different values are not equal
	c.Assert(errors.Is(err0, err1), check.IsFalse)
	c.Assert(errors.Is(err1, err0), check.IsFalse)

	// errors with different types are not equal
	err0.Value = 2
	c.Assert(errors.Is(err0, err1), check.IsFalse)
	c.Assert(errors.Is(err1, err0), check.IsFalse)

	err2 := &ValidateErr{
		Type:   TypeMismatch,
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value: map[string]any{
			"key1": 1,
			"key2": "2",
		},
	}
	c.Assert(errors.Is(err2, ErrValidate), check.IsTrue)
	c.Assert(errors.Is(err1, err2), check.IsFalse)
	c.Assert(errors.Is(err2, err1), check.IsFalse)

	err3 := &ValidateErr{
		Type:   TypeMismatch,
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  []float64{1.0, 2.0},
	}
	c.Assert(errors.Is(err3, ErrValidate), check.IsTrue)
	// different values are not equal
	c.Assert(errors.Is(err2, err3), check.IsFalse)
	c.Assert(errors.Is(err3, err2), check.IsFalse)

	err4 := &ValidateErr{
		Type:   TypeMismatch,
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  nil,
	}
	c.Assert(errors.Is(err4, ErrValidate), check.IsTrue)
	// nil Value matches any error if other fields are with the same values
	c.Assert(errors.Is(err3, err4), check.IsTrue)
	c.Assert(errors.Is(err4, err3), check.IsFalse)

	err4.Value = 0
	c.Assert(errors.Is(err4, ErrValidate), check.IsTrue)
	c.Assert(errors.Is(err3, err4), check.IsFalse)
	c.Assert(errors.Is(err4, err3), check.IsFalse)
}
