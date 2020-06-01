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

package report

import (
	"bytes"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/pingcap/tiup/pkg/localdata"
	tiuptele "github.com/pingcap/tiup/pkg/telemetry"
)

// Enable return true if we enable telemetry.
func Enable() bool {
	s := os.Getenv(localdata.EnvNameTelemetryStatus)
	status := tiuptele.Status(s)
	return status == tiuptele.EnableStatus
}

// UUID return telemetry uuid.
func UUID() string {
	return os.Getenv(localdata.EnvNameTelemetryUUID)
}

// NodeInfoFromText get telemetry.NodeInfo from the text.
func NodeInfoFromText(text string) (info *tiuptele.NodeInfo, err error) {
	info = new(tiuptele.NodeInfo)
	err = proto.UnmarshalText(text, info)
	if err != nil {
		return nil, err
	}

	return
}

// NodeInfoToText get telemetry.NodeInfo in text.
func NodeInfoToText(info *tiuptele.NodeInfo) (text string, err error) {
	buf := new(bytes.Buffer)
	err = proto.MarshalText(buf, info)
	if err != nil {
		return
	}
	text = buf.String()

	return
}
