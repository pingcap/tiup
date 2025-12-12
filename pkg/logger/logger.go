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

package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitGlobalLogger initializes zap global logger.
// Default level is Debug.
func InitGlobalLogger() {
	core := zapcore.NewTee(
		newAuditLogCore(),
		newDebugLogCore(),
	)
	level := zapcore.DebugLevel
	atomic := zap.NewAtomicLevelAt(level)
	logger := zap.New(core, zap.IncreaseLevel(atomic))
	zap.ReplaceGlobals(logger)
}
