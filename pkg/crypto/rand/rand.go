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
	"crypto/rand"
	"io"
	"math/big"
)

// rand provides a simple wrap of "crypto/rand".

// Reader is a global random number source
var Reader io.Reader = &cryptoRandReader{}

type cryptoRandReader struct{}

func (c *cryptoRandReader) Read(b []byte) (int, error) {
	return rand.Read(b)
}

// Int wraps Int63n
func Int() int {
	val := Int63n(int64(int(^uint(0) >> 1)))
	return int(val)
}

// Intn wraps Int63n
func Intn(n int) int {
	if n <= 0 {
		panic("argument to Intn must be positive")
	}
	val := Int63n(int64(n))
	return int(val)
}

// Int63n wraps rand.Int
func Int63n(n int64) int64 {
	if n <= 0 {
		panic("argument to Int63n must be positive")
	}
	val, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		panic(err)
	}
	return val.Int64()
}
