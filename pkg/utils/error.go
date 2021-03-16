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

import "fmt"

var (
	// ErrValidateChecksum is an empty HashValidationErr object, useful for type checking
	ErrValidateChecksum = &HashValidationErr{}
)

// HashValidationErr is the error indicates a failed hash validation
type HashValidationErr struct {
	cipher string
	expect string // expected hash
	actual string // input hash
}

// Error implements the error interface
func (e *HashValidationErr) Error() string {
	return fmt.Sprintf(
		"%s checksum mismatch, expect: %v, got: %v",
		e.cipher, e.expect, e.actual,
	)
}

// Unwrap implements the error interface
func (e *HashValidationErr) Unwrap() error { return nil }

// Is implements the error interface
func (e *HashValidationErr) Is(target error) bool {
	t, ok := target.(*HashValidationErr)
	if !ok {
		return false
	}

	return (e.cipher == t.cipher || t.cipher == "") &&
		(e.expect == t.expect || t.expect == "") &&
		(e.actual == t.actual || t.actual == "")
}
