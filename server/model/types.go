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

package model

import (
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
)

// ComponentManifest represents xxx.json
type ComponentManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Component `json:"signed"`
}

// RootManifest represents root.json
type RootManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Root `json:"signed"`
}

// IndexManifest represents index.json
type IndexManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Index `json:"signed"`
}

// KeyOwner returns the owner or nil for a given keyid
func (m *IndexManifest) KeyOwner(keyid string) (string, *v1manifest.Owner) {
	for on := range m.Signed.Owners {
		for k := range m.Signed.Owners[on].Keys {
			if k == keyid {
				o := m.Signed.Owners[on]
				return on, &o
			}
		}
	}
	return "", nil
}

// SnapshotManifest represents snapshot.json
type SnapshotManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Snapshot `json:"signed"`
}

// TimestampManifest represents timestamp.json
type TimestampManifest struct {
	// Signatures value
	Signatures []v1manifest.Signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Timestamp `json:"signed"`
}
