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

package tui

// A set of predefined color palettes. You should only reference a color in this palette so that a color
// change can take effect globally.

import (
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	// ColorErrorMsg is the ansi color formatter for error messages
	ColorErrorMsg = color.New(color.FgHiRed)
	// ColorSuccessMsg is the ansi color formatter for success messages
	ColorSuccessMsg = color.New(color.FgHiGreen)
	// ColorWarningMsg is the ansi color formatter for warning messages
	ColorWarningMsg = color.New(color.FgHiYellow)
	// ColorCommand is the ansi color formatter for commands
	ColorCommand = color.New(color.FgHiBlue, color.Bold)
	// ColorKeyword is the ansi color formatter for cluster name
	ColorKeyword = color.New(color.FgHiBlue, color.Bold)
)

func newColorizeFn(c *color.Color) func() string {
	const sep = "----"
	seq := c.Sprint(sep)
	if len(seq) == len(sep) {
		return func() string {
			return ""
		}
	}
	colorSeq := strings.Split(seq, sep)[0]
	return func() string {
		return colorSeq
	}
}

func newColorResetFn() func() string {
	const sep = "----"
	seq := color.New(color.FgWhite).Sprint(sep)
	if len(seq) == len(sep) {
		return func() string {
			return ""
		}
	}
	colorResetSeq := strings.Split(seq, sep)[1]
	return func() string {
		return colorResetSeq
	}
}

// AddColorFunctions invokes callback for each colorize functions.
func AddColorFunctions(addCallback func(string, any)) {
	addCallback("ColorErrorMsg", newColorizeFn(ColorErrorMsg))
	addCallback("ColorSuccessMsg", newColorizeFn(ColorSuccessMsg))
	addCallback("ColorWarningMsg", newColorizeFn(ColorWarningMsg))
	addCallback("ColorCommand", newColorizeFn(ColorCommand))
	addCallback("ColorKeyword", newColorizeFn(ColorKeyword))
	addCallback("ColorReset", newColorResetFn())
}

// AddColorFunctionsForCobra adds colorize functions to cobra, so that they can be used in usage or help.
func AddColorFunctionsForCobra() {
	AddColorFunctions(func(name string, f any) {
		cobra.AddTemplateFunc(name, f)
	})
}
