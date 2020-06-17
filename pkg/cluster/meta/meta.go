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
	"fmt"
	"reflect"

	"github.com/joomcode/errorx"
)

var (
	errNS = errorx.NewNamespace("meta")
	// ValidateErr is an empty validateErr object, useful for type checking
	ValidateErr = &validateErr{}
)

// error types
const (
	errTypeConflict = "conflict"
	errTypeMismatch = "mismatch"
)

// validateErr is the error when meta validation fails with conflicts
type validateErr struct {
	ty     string      // conflict type
	target string      // conflict target
	value  interface{} // conflict value
	one    string      // object 1
	two    string      // object 2
}

// Error implements the error interface
func (e *validateErr) Error() string {
	return fmt.Sprintf("%s %s for '%v' between '%s' and '%s'", e.target, e.ty, e.value, e.one, e.two)
}

// Unwrap implements the error interface
func (e *validateErr) Unwrap() error { return nil }

// Is implements the error interface
func (e *validateErr) Is(target error) bool {
	t, ok := target.(*validateErr)
	if !ok {
		return false
	}

	// check for interface value seperately
	if e.value != nil && t.value != nil &&
		(!reflect.ValueOf(e.value).IsValid() && !reflect.ValueOf(t.value).IsValid()) {
		return false
	}
	// not supporting non-comparable values for now
	if e.value != nil && t.value != nil &&
		!(reflect.TypeOf(e.value).Comparable() && reflect.TypeOf(t.value).Comparable()) {
		return false
	}
	return (e.ty == t.ty || t.ty == "") &&
		(e.target == t.target || t.target == "") &&
		(e.value == t.value || t.value == nil || reflect.ValueOf(t.value).IsZero()) &&
		(e.one == t.one || t.one == "") &&
		(e.two == t.two || t.two == "")
}
