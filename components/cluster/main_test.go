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

package main

import (
	"os"
	"strings"
	"testing"
)

// To build:
// see build_integration_test in Makefile
// To run:
// tiup-cluster.test  -test.coverprofile={file} __DEVEL--i-heard-you-like-tests

func TestMain(t *testing.T) {
	var (
		args []string
		run  bool
	)

	for _, arg := range os.Args {
		switch {
		case arg == "__DEVEL--i-heard-you-like-tests":
			run = true
		case strings.HasPrefix(arg, "-test"):
		case strings.HasPrefix(arg, "__DEVEL"):
		default:
			args = append(args, arg)
		}
	}
	os.Args = args

	// fmt.Println(os.Args)

	if run {
		main()
	}
}
