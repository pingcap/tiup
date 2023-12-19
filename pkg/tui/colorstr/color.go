// Copyright 2024 PingCAP, Inc.
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

// Package colorstr interprets the input format containing color tokens like `[blue]hello [red]world`
// as the text "hello world" in two colors.
//
// Just like tokens in the fmt package (e.g. '%s'), color tokens will only be effective when specified
// as the format parameter. Tokens not in the format parameter will not be interpreted.
//
//	colorstr.DefaultTokens.Printf("[blue]hello")   ==> (a blue hello)
//	colorstr.DefaultTokens.Printf("[ahh]")         ==> "[ahh]"
//
// Color tokens in the Print arguments will never be interpreted. It can be useful to pass user inputs there.
package colorstr

import (
	"fmt"
	"io"

	"github.com/mitchellh/colorstring"
)

type colorTokens struct {
	colorstring.Colorize
}

// Note: Print, Println, Fprint, Fprintln are intentionally not implemented, as we would like to
// limit the usage of color token to be only placed in the "format" part.

// Printf is a convenience wrapper for fmt.Printf with support for color codes.
// Only color codes in the format param will be respected.
func (c colorTokens) Printf(format string, a ...any) (n int, err error) {
	return fmt.Printf(c.Color(format), a...)
}

// Fprintf is a convenience wrapper for fmt.Fprintf with support for color codes.
// Only color codes in the format param will be respected.
func (c colorTokens) Fprintf(w io.Writer, format string, a ...any) (n int, err error) {
	return fmt.Fprintf(w, c.Color(format), a...)
}

// Sprintf is a convenience wrapper for fmt.Sprintf with support for color codes.
// Only color codes in the format param will be respected.
func (c colorTokens) Sprintf(format string, a ...any) string {
	return fmt.Sprintf(c.Color(format), a...)
}

// DefaultTokens uses default color tokens.
var DefaultTokens = (func() colorTokens {
	// TODO: Respect NO_COLOR env
	// TODO: Add more color tokens here
	colors := make(map[string]string)
	for k, v := range colorstring.DefaultColors {
		colors[k] = v
	}
	return colorTokens{
		Colorize: colorstring.Colorize{
			Colors:  colors,
			Disable: false,
			Reset:   true,
		},
	}
})()

// Printf is a convenience wrapper for fmt.Printf with support for color codes.
// Only color codes in the format param will be respected.
func Printf(format string, a ...any) (n int, err error) {
	return DefaultTokens.Printf(format, a...)
}

// Fprintf is a convenience wrapper for fmt.Fprintf with support for color codes.
// Only color codes in the format param will be respected.
func Fprintf(w io.Writer, format string, a ...any) (n int, err error) {
	return DefaultTokens.Fprintf(w, format, a...)
}

// Sprintf is a convenience wrapper for fmt.Sprintf with support for color codes.
// Only color codes in the format param will be respected.
func Sprintf(format string, a ...any) string {
	return DefaultTokens.Sprintf(format, a...)
}
