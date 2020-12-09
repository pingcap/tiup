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
	"os"

	"go.uber.org/zap"
)

// Debugf output the debug message to console
func Debugf(format string, args ...interface{}) {
	zap.L().Debug(fmt.Sprintf(format, args...))
}

// Infof output the log message to console
// Deprecated: Use zap.L().Info() instead
func Infof(format string, args ...interface{}) {
	zap.L().Info(fmt.Sprintf(format, args...))
	fmt.Printf(format+"\n", args...)
}

// Warnf output the warning message to console
// Deprecated: Use zap.L().Warn() instead
func Warnf(format string, args ...interface{}) {
	zap.L().Warn(fmt.Sprintf(format, args...))
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Errorf output the error message to console
// Deprecated: Use zap.L().Error() instead
func Errorf(format string, args ...interface{}) {
	zap.L().Error(fmt.Sprintf(format, args...))
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
}
