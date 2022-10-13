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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
)

var (
	outputFmt = DisplayModeDefault // global output format of logger

	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// DisplayMode control the output format
type DisplayMode int

// display modes
const (
	DisplayModeDefault DisplayMode = iota // default is the interactive output
	DisplayModePlain                      // plain text
	DisplayModeJSON                       // JSON
)

func fmtDisplayMode(m string) DisplayMode {
	var dp DisplayMode
	switch strings.ToLower(m) {
	case "json":
		dp = DisplayModeJSON
	case "plain", "text":
		dp = DisplayModePlain
	default:
		dp = DisplayModeDefault
	}
	return dp
}

func printLog(w io.Writer, mode DisplayMode, level, format string, args ...any) {
	switch mode {
	case DisplayModeJSON:
		obj := struct {
			Level string `json:"level"`
			Msg   string `json:"message"`
		}{
			Level: level,
			Msg:   fmt.Sprintf(format, args...),
		}
		data, err := json.Marshal(obj)
		if err != nil {
			_, _ = fmt.Fprintf(w, "{\"error\":\"%s\"}", err)
			return
		}
		_, _ = fmt.Fprint(w, string(data)+"\n")
	default:
		_, _ = fmt.Fprintf(w, format+"\n", args...)
	}
}

// SetDisplayMode changes the global output format of logger
func SetDisplayMode(m DisplayMode) {
	outputFmt = m
}

// GetDisplayMode returns the current global output format
func GetDisplayMode() DisplayMode {
	return outputFmt
}

// SetDisplayModeFromString changes the global output format of logger
func SetDisplayModeFromString(m string) {
	outputFmt = fmtDisplayMode(m)
}

// Debugf output the debug message to console
func Debugf(format string, args ...any) {
	zap.L().Debug(fmt.Sprintf(format, args...))
}

// Infof output the log message to console
// Deprecated: Use zap.L().Info() instead
func Infof(format string, args ...any) {
	zap.L().Info(fmt.Sprintf(format, args...))
	printLog(stdout, outputFmt, "info", format, args...)
}

// Warnf output the warning message to console
// Deprecated: Use zap.L().Warn() instead
func Warnf(format string, args ...any) {
	zap.L().Warn(fmt.Sprintf(format, args...))
	printLog(stderr, outputFmt, "warn", format, args...)
}

// Errorf output the error message to console
// Deprecated: Use zap.L().Error() instead
func Errorf(format string, args ...any) {
	zap.L().Error(fmt.Sprintf(format, args...))
	printLog(stderr, outputFmt, "error", format, args...)
}

// SetStdout redirect stdout to a custom writer
func SetStdout(w io.Writer) {
	stdout = w
}

// SetStderr redirect stderr to a custom writer
func SetStderr(w io.Writer) {
	stderr = w
}
