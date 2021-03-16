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

	. "github.com/pingcap/check"
)

type errSuite struct {
}

var _ = Suite(&errSuite{})

func (s *errSuite) TestHashValidationErr(c *C) {
	err0 := &HashValidationErr{
		cipher: "sha256",
		expect: "hash111",
		actual: "hash222",
	}
	// identical errors are equal
	c.Assert(errors.Is(err0, err0), IsTrue)
	c.Assert(errors.Is(ErrValidateChecksum, ErrValidateChecksum), IsTrue)
	c.Assert(errors.Is(ErrValidateChecksum, &HashValidationErr{}), IsTrue)
	c.Assert(errors.Is(&HashValidationErr{}, ErrValidateChecksum), IsTrue)
	// not equal for different error types
	c.Assert(errors.Is(err0, errors.New("")), IsFalse)
	// default Value matches any error
	c.Assert(errors.Is(err0, ErrValidateChecksum), IsTrue)
	// error with values are not matching default ones
	c.Assert(errors.Is(ErrValidateChecksum, err0), IsFalse)

	err1 := &HashValidationErr{
		cipher: "sha256",
		expect: "hash111",
		actual: "hash222",
	}
	c.Assert(errors.Is(err1, ErrValidateChecksum), IsTrue)
	// errors with same values are equal
	c.Assert(errors.Is(err0, err1), IsTrue)
	c.Assert(errors.Is(err1, err0), IsTrue)
	// errors with different ciphers are not equal
	err1.cipher = "sha512"
	c.Assert(errors.Is(err0, err1), IsFalse)
	c.Assert(errors.Is(err1, err0), IsFalse)
	// errors with different expected hashes are not equal
	err1.cipher = err0.cipher
	c.Assert(errors.Is(err0, err1), IsTrue)
	err1.expect = "hash1112"
	c.Assert(errors.Is(err0, err1), IsFalse)
	c.Assert(errors.Is(err1, err0), IsFalse)
	// errors with different actual hashes are not equal
	err1.expect = err0.expect
	c.Assert(errors.Is(err0, err1), IsTrue)
	err1.actual = "hash2223"
	c.Assert(errors.Is(err0, err1), IsFalse)
	c.Assert(errors.Is(err1, err0), IsFalse)
}
