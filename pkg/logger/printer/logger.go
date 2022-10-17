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
	"io"
	"os"

	"go.uber.org/zap"
)

// ContextKey is key used to store values in context
type ContextKey string

// ContextKeyLogger is the key used for logger stored in context
const ContextKeyLogger ContextKey = "logger"

// Logger is a set of fuctions writing output to custom writters, but still
// using the global zap logger as our default config does not writes everything
// to a memory buffer.
// TODO: use also separate zap loggers
type Logger struct {
	outputFmt DisplayMode //  output format of logger

	stdout io.Writer
	stderr io.Writer
}

// NewLogger creates a Logger with default settings
func NewLogger(m string) *Logger {
	return &Logger{
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		outputFmt: fmtDisplayMode(m),
	}
}

// SetStdout redirect stdout to a custom writer
func (l *Logger) SetStdout(w io.Writer) {
	l.stdout = w
}

// SetStderr redirect stderr to a custom writer
func (l *Logger) SetStderr(w io.Writer) {
	l.stderr = w
}

// SetDisplayMode changes the output format of logger
func (l *Logger) SetDisplayMode(m DisplayMode) {
	l.outputFmt = m
}

// SetDisplayModeFromString changes the output format of logger
func (l *Logger) SetDisplayModeFromString(m string) {
	l.outputFmt = fmtDisplayMode(m)
}

// GetDisplayMode returns the current output format
func (l *Logger) GetDisplayMode() DisplayMode {
	return l.outputFmt
}

// Debugf output the debug message to console
func (l *Logger) Debugf(format string, args ...any) {
	zap.L().Debug(fmt.Sprintf(format, args...))
}

// Infof output the log message to console
func (l *Logger) Infof(format string, args ...any) {
	zap.L().Info(fmt.Sprintf(format, args...))
	printLog(l.stdout, l.outputFmt, "info", format, args...)
}

// Warnf output the warning message to console
func (l *Logger) Warnf(format string, args ...any) {
	zap.L().Warn(fmt.Sprintf(format, args...))
	printLog(l.stderr, l.outputFmt, "warn", format, args...)
}

// Errorf output the error message to console
func (l *Logger) Errorf(format string, args ...any) {
	zap.L().Error(fmt.Sprintf(format, args...))
	printLog(l.stderr, l.outputFmt, "error", format, args...)
}
