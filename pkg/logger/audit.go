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
	"io/ioutil"
	"os"
	"strings"
	"time"

	utils2 "github.com/pingcap/tiup/pkg/utils"

	"github.com/pingcap/tiup/pkg/base52"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var auditEnabled atomic.Bool
var auditBuffer *bytes.Buffer

// EnableAuditLog enables audit log.
func EnableAuditLog() {
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
	auditDir := spec.ProfilePath(spec.TiOpsAuditDir)
	if err := utils2.CreateDir(auditDir); err != nil {
		zap.L().Warn("Create audit directory failed", zap.Error(err))
	} else {
		auditFilePath := spec.ProfilePath(spec.TiOpsAuditDir, base52.Encode(time.Now().Unix()))
		err := ioutil.WriteFile(auditFilePath, auditBuffer.Bytes(), os.ModePerm)
		if err != nil {
			zap.L().Warn("Write audit log file failed", zap.Error(err))
		}
		auditBuffer.Reset()
	}
}
