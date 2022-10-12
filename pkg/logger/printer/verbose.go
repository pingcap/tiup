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

package logprinter

import (
	"fmt"
	"os"
	"strings"
)

var verbose bool

func init() {
	v := strings.ToLower(os.Getenv("TIUP_VERBOSE"))
	verbose = v == "1" || v == "enable"
}

// Verbose logs verbose messages
func Verbose(format string, args ...any) {
	if !verbose {
		return
	}
	fmt.Fprintln(stderr, "Verbose:", fmt.Sprintf(format, args...))
}

// Verbose logs verbose messages
func (l *Logger) Verbose(format string, args ...any) {
	if !verbose {
		return
	}
	fmt.Fprintln(l.stderr, "Verbose:", fmt.Sprintf(format, args...))
}
