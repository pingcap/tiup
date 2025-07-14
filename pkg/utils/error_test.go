// Copyright 2021 PingCAP, Inc.
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

package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashValidationErr(t *testing.T) {
	err0 := &HashValidationErr{
		cipher: "sha256",
		expect: "hash111",
		actual: "hash222",
	}
	// identical errors are equal
	require.True(t, errors.Is(err0, err0))
	require.True(t, errors.Is(ErrValidateChecksum, ErrValidateChecksum))
	require.True(t, errors.Is(ErrValidateChecksum, &HashValidationErr{}))
	require.True(t, errors.Is(&HashValidationErr{}, ErrValidateChecksum))
	// not equal for different error types
	require.False(t, errors.Is(err0, errors.New("")))
	// default Value matches any error
	require.True(t, errors.Is(err0, ErrValidateChecksum))
	// error with values are not matching default ones
	require.False(t, errors.Is(ErrValidateChecksum, err0))

	err1 := &HashValidationErr{
		cipher: "sha256",
		expect: "hash111",
		actual: "hash222",
	}
	require.True(t, errors.Is(err1, ErrValidateChecksum))
	// errors with same values are equal
	require.True(t, errors.Is(err0, err1))
	require.True(t, errors.Is(err1, err0))
	// errors with different ciphers are not equal
	err1.cipher = "sha512"
	require.False(t, errors.Is(err0, err1))
	require.False(t, errors.Is(err1, err0))
	// errors with different expected hashes are not equal
	err1.cipher = err0.cipher
	require.True(t, errors.Is(err0, err1))
	err1.expect = "hash1112"
	require.False(t, errors.Is(err0, err1))
	require.False(t, errors.Is(err1, err0))
	// errors with different actual hashes are not equal
	err1.expect = err0.expect
	require.True(t, errors.Is(err0, err1))
	err1.actual = "hash2223"
	require.False(t, errors.Is(err0, err1))
	require.False(t, errors.Is(err1, err0))
}
