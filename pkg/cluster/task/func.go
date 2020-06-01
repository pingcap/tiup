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

package task

// Func wrap a closure.
type Func struct {
	name string
	fn   func(ctx *Context) error
}

// NewFunc create a Func task
func NewFunc(name string, fn func(ctx *Context) error) *Func {
	return &Func{
		name: name,
		fn:   fn,
	}
}

// Execute implements the Task interface
func (m *Func) Execute(ctx *Context) error {
	return m.fn(ctx)
}

// Rollback implements the Task interface
func (m *Func) Rollback(_ *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (m *Func) String() string {
	return m.name
}
