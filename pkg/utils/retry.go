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

package utils

// Retry when the when func returns true
func Retry(f func() error, when func(error) bool) error {
	e := f()
	if e == nil {
		return nil
	}
	if when == nil {
		return Retry(f, nil)
	} else if when(e) {
		return Retry(f, when)
	}
	return e
}
