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
	"strconv"
	"strings"

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
