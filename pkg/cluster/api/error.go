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

import "fmt"

var (
	// ErrNoStore is an empty NoStoreErr object, useful for type checking
	ErrNoStore = &NoStoreErr{}
)

// NoStoreErr is the error that no store matching address can be found in PD
type NoStoreErr struct {
	addr string
}

// Error implement the error interface
func (e *NoStoreErr) Error() string {
	return fmt.Sprintf("no store matching address \"%s\" found", e.addr)
}

// Is implements the error interface
func (e *NoStoreErr) Is(target error) bool {
	t, ok := target.(*NoStoreErr)
	if !ok {
		return false
	}

	return e.addr == t.addr || t.addr == ""
}
