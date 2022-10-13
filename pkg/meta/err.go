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
)

var (
	// ErrValidate is an empty ValidateErr object, useful for type checking
	ErrValidate = &ValidateErr{}
)

// error types
const (
	TypeConflict = "conflict"
	TypeMismatch = "mismatch"
)

// ValidateErr is the error when meta validation fails with conflicts
type ValidateErr struct {
	Type   string // conflict type
	Target string // conflict Target
	Value  any    // conflict Value
	LHS    string // object 1
	RHS    string // object 2
}

// Error implements the error interface
func (e *ValidateErr) Error() string {
	return fmt.Sprintf("%s %s for '%v' between '%s' and '%s'", e.Target, e.Type, e.Value, e.LHS, e.RHS)
}

// Unwrap implements the error interface
func (e *ValidateErr) Unwrap() error { return nil }

// Is implements the error interface
func (e *ValidateErr) Is(target error) bool {
	t, ok := target.(*ValidateErr)
	if !ok {
		return false
	}

	// check for interface Value separately
	if e.Value != nil && t.Value != nil &&
		(!reflect.ValueOf(e.Value).IsValid() && !reflect.ValueOf(t.Value).IsValid()) {
		return false
	}
	// not supporting non-comparable values for now
	if e.Value != nil && t.Value != nil &&
		!(reflect.TypeOf(e.Value).Comparable() && reflect.TypeOf(t.Value).Comparable()) {
		return false
	}
	return (e.Type == t.Type || t.Type == "") &&
		(e.Target == t.Target || t.Target == "") &&
		(e.Value == t.Value || t.Value == nil || reflect.ValueOf(t.Value).IsZero()) &&
		(e.LHS == t.LHS || t.LHS == "") &&
		(e.RHS == t.RHS || t.RHS == "")
}
