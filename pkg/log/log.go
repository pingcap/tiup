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

package log

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

var output io.Writer = os.Stdout

// SetOutput overwrites the default output writer
func SetOutput(w io.Writer) {
	output = w
}

// Output print the message to output
func Output(s string) {
	fmt.Fprintln(output, s)
}

// Debugf output the debug message to console
func Debugf(format string, args ...interface{}) {
	Output(color.CyanString(format, args...))
}

// Infof output the log message to console
func Infof(format string, args ...interface{}) {
	Output(color.GreenString(format, args...))
}

// Warnf output the warning message to console
func Warnf(format string, args ...interface{}) {
	Output(color.YellowString(format, args...))
}

// Errorf output the error message to console
func Errorf(format string, args ...interface{}) {
	Output(color.RedString(format, args...))
}
