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

	"github.com/stretchr/testify/require"
)

func TestValidateErrIs(t *testing.T) {
	err0 := &ValidateErr{
		Type:   "dummy",
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  1,
	}
	// identical errors are equal
	require.True(t, errors.Is(err0, err0))
	require.True(t, errors.Is(ErrValidate, ErrValidate))
	require.True(t, errors.Is(ErrValidate, &ValidateErr{}))
	require.True(t, errors.Is(&ValidateErr{}, ErrValidate))
	// not equal for different error types
	require.False(t, errors.Is(err0, errors.New("")))
	// default Value matches any error
	require.True(t, errors.Is(err0, ErrValidate))
	// error with values are not matching default ones
	require.False(t, errors.Is(ErrValidate, err0))

	err1 := &ValidateErr{
		Type:   TypeConflict,
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  2,
	}
	require.True(t, errors.Is(err1, ErrValidate))
	// errors with different values are not equal
	require.False(t, errors.Is(err0, err1))
	require.False(t, errors.Is(err1, err0))

	// errors with different types are not equal
	err0.Value = 2
	require.False(t, errors.Is(err0, err1))
	require.False(t, errors.Is(err1, err0))

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
	require.True(t, errors.Is(err2, ErrValidate))
	require.False(t, errors.Is(err1, err2))
	require.False(t, errors.Is(err2, err1))

	err3 := &ValidateErr{
		Type:   TypeMismatch,
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  []float64{1.0, 2.0},
	}
	require.True(t, errors.Is(err3, ErrValidate))
	// different values are not equal
	require.False(t, errors.Is(err2, err3))
	require.False(t, errors.Is(err3, err2))

	err4 := &ValidateErr{
		Type:   TypeMismatch,
		Target: "test",
		LHS:    "LHS",
		RHS:    "RHS",
		Value:  nil,
	}
	require.True(t, errors.Is(err4, ErrValidate))
	// nil Value matches any error if other fields are with the same values
	require.True(t, errors.Is(err3, err4))
	require.False(t, errors.Is(err4, err3))

	err4.Value = 0
	require.True(t, errors.Is(err4, ErrValidate))
	require.False(t, errors.Is(err3, err4))
	require.False(t, errors.Is(err4, err3))
}
