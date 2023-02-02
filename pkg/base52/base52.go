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

package base52

import (
	"fmt"
	"strings"
)

const (
	space = "0123456789bcdfghjkmnpqrstvwxyzBCDFGHJKLMNPQRSTVWXYZ"
	base  = len(space)
)

// Encode returns a string by encoding the id over a 51 characters space
func Encode(id int64) string {
	var short []byte
	for id > 0 {
		i := id % int64(base)
		short = append(short, space[i])
		id /= int64(base)
	}
	for i, j := 0, len(short)-1; i < j; i, j = i+1, j-1 {
		short[i], short[j] = short[j], short[i]
	}
	return string(short)
}

// Decode will decode the string and return the id
// The input string should be a valid one with only characters in the space
func Decode(encoded string) (int64, error) {
	if len(encoded) != len([]rune(encoded)) {
		return 0, fmt.Errorf("invalid encoded string: '%s'", encoded)
	}
	var id int64
	for i := 0; i < len(encoded); i++ {
		idx := strings.IndexByte(space, encoded[i])
		if idx < 0 {
			return 0, fmt.Errorf("invalid encoded string: '%s' contains invalid character", encoded)
		}
		id = id*int64(base) + int64(idx)
	}
	return id, nil
}
