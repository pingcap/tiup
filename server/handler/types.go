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

package handler

import (
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
)

type simpleResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type componentManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Component `json:"signed"`
}

type indexManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Index `json:"signed"`
}

type snapshotManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Snapshot `json:"signed"`
}

type timestampManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Timestamp `json:"signed"`
}
