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

package rand

import (
	cr "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
)

var (
	// Reader is a global random number source
	Reader *rand.Rand
)

func init() {
	src := make([]byte, 8)
	if _, err := cr.Read(src); err != nil {
		panic(fmt.Sprintf("initial random: %s", err.Error()))
	}
	seed := binary.BigEndian.Uint64(src)
	Reader = rand.New(rand.NewSource(int64(seed)))
}

// Int wraps Rand.Int
func Int() int {
	return Reader.Int()
}

// Intn wraps Rand.Intn
func Intn(n int) int {
	return Reader.Intn(n)
}

// Int63n wraps Rand.Int63n
func Int63n(n int64) int64 {
	return Reader.Int63n(n)
}

// Read wraps Rand.Read
func Read(b []byte) (int, error) {
	return Reader.Read(b)
}
