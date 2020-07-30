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
	"bytes"
	"os"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/audit"
	utils2 "github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var auditEnabled atomic.Bool
var auditBuffer *bytes.Buffer
var auditDir string

// EnableAuditLog enables audit log.
func EnableAuditLog(dir string) {
	auditDir = dir
	auditEnabled.Store(true)
}

// DisableAuditLog disables audit log.
func DisableAuditLog() {
	auditEnabled.Store(false)
}

func newAuditLogCore() zapcore.Core {
	auditBuffer = bytes.NewBufferString(strings.Join(os.Args, " ") + "\n")
	encoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	return zapcore.NewCore(encoder, zapcore.Lock(zapcore.AddSync(auditBuffer)), zapcore.InfoLevel)
}

// OutputAuditLogIfEnabled outputs audit log if enabled.
func OutputAuditLogIfEnabled() {
	if !auditEnabled.Load() {
		return
	}

	if err := utils2.CreateDir(auditDir); err != nil {
		zap.L().Warn("Create audit directory failed", zap.Error(err))
		return
	}

	err := audit.OutputAuditLog(auditDir, auditBuffer.Bytes())
	if err != nil {
		zap.L().Warn("Write audit log file failed", zap.Error(err))
	}
	auditBuffer.Reset()
}
