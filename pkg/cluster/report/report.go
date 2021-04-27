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
	"runtime"

	"github.com/gogo/protobuf/proto"
	"github.com/pingcap/tiup/pkg/localdata"
	tiuptele "github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/version"
)

// Enabled return true if we enable telemetry.
func Enabled() bool {
	s := os.Getenv(localdata.EnvNameTelemetryStatus)
	status := tiuptele.Status(s)
	return status == tiuptele.EnableStatus
}

// TelemetryUUID return telemetry uuid.
func TelemetryUUID() string {
	return os.Getenv(localdata.EnvNameTelemetryUUID)
}

// TelemetrySecret return telemetry uuid.
func TelemetrySecret() string {
	return os.Getenv(localdata.EnvNameTelemetrySecret)
}

// TiUPMeta returns metadata of TiUP Cluster itself
func TiUPMeta() *tiuptele.TiUPInfo {
	return &tiuptele.TiUPInfo{
		TiUPVersion:      os.Getenv(localdata.EnvNameTiUPVersion),
		ComponentVersion: version.NewTiUPVersion().SemVer(),
		GitCommit:        version.NewTiUPBuildInfo().GitHash,
		Os:               runtime.GOOS,
		Arch:             runtime.GOARCH,
	}
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
