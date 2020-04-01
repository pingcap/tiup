package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitGlobalLogger initializes zap global logger.
func InitGlobalLogger() {
	core := zapcore.NewTee(
		newAuditLogCore(),
		newDebugLogCore(),
	)
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)
}
