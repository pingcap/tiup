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
	"os"
	"sync/atomic"

	"github.com/mitchellh/colorstring"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
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

var colorEnabled atomic.Bool

func init() {
	// Enable color when either stdout or stderr supports it. This avoids surprising
	// "no color on stderr" behavior when stdout is piped but stderr is still a TTY.
	colorEnabled.Store(tuiterm.ResolveFile(os.Stdout).Color || tuiterm.ResolveFile(os.Stderr).Color)
}

// SetColorEnabled sets whether ANSI styling output is enabled globally.
//
// This matches the behavior of common CLI color libraries (e.g. github.com/fatih/color):
// the decision is made once and then reused across all output.
func SetColorEnabled(enabled bool) {
	colorEnabled.Store(enabled)
}

// ColorEnabled reports whether ANSI styling output is enabled globally.
func ColorEnabled() bool {
	return colorEnabled.Load()
}

func tokens() colorTokens {
	tokens := DefaultTokens
	tokens.Disable = !ColorEnabled()
	return tokens
}

// Printf is a convenience wrapper for fmt.Printf with support for color codes.
// Only color codes in the format param will be respected.
func Printf(format string, a ...any) (n int, err error) {
	return Fprintf(os.Stdout, format, a...)
}

// Fprintf is a convenience wrapper for fmt.Fprintf with support for color codes.
// Only color codes in the format param will be respected.
func Fprintf(w io.Writer, format string, a ...any) (n int, err error) {
	return tokens().Fprintf(w, format, a...)
}

// Sprintf is a convenience wrapper for fmt.Sprintf with support for color codes.
// Only color codes in the format param will be respected.
func Sprintf(format string, a ...any) string {
	return tokens().Sprintf(format, a...)
}
