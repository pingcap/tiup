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

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// JoinInt joins a slice of int to string
func JoinInt(nums []int, delim string) string {
	result := ""
	for _, i := range nums {
		result += strconv.Itoa(i)
		result += delim
	}
	return strings.TrimSuffix(result, delim)
}

// IsFlagSetByUser check if the a flag is set by user explicitly
func IsFlagSetByUser(flagSet *pflag.FlagSet, flagName string) bool {
	setByUser := false
	flagSet.Visit(func(f *pflag.Flag) {
		if f.Name == flagName {
			setByUser = true
		}
	})
	return setByUser
}

// MustAtoI calls strconv.Atoi and ignores error
func MustAtoI(a string) int {
	v, _ := strconv.Atoi(a)
	return v
}

// Base62Tag returns a tag based on time
func Base62Tag() string {
	const base = 62
	const sets = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 0)
	num := time.Now().UnixNano() / int64(time.Millisecond)
	for num > 0 {
		r := math.Mod(float64(num), float64(base))
		num /= base
		b = append([]byte{sets[int(r)]}, b...)
	}
	return string(b)
}

// Ternary operator
func Ternary(condition bool, a, b interface{}) interface{} {
	if condition {
		return a
	}
	return b
}
