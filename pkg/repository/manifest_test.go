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

package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadTimestamp(t *testing.T) {
	_, err := readTimestampManifest(strings.NewReader(`{
		"signatures":[
			{
				"keyid": "TODO",
				"sig": "TODO"
			}
		],
		"signed": {
			"_type": "timestamp",
			"spec_version": "0.1.0",
			"expires": "2220-05-11T04:51:08Z",
			"version": 43,
			"meta": {
				"snapshot.json": {
					"hashes": {
						"sha256": "TODO"
					},
					"length": 515
				}
			}
		}
	}`), nil)
	assert.Nil(t, err)
}

func TestEmptyManifest(t *testing.T) {
	var ts timestamp
	err := readManifest(strings.NewReader(""), &ts, nil)
	assert.NotNil(t, err)
}

func TestWriteManifest(t *testing.T) {
	ts := timestamp{Meta: map[string]fileHash{"snapshot.json": {
		Hashes: map[string]string{"TODO": "TODO"},
		Length: 0,
	}},
		signedBase: signedBase{
			Ty:          "timestamp",
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     10,
		}}
	var out strings.Builder
	err := writeManifest(&out, &ts)
	assert.Nil(t, err)
	ts2, err := readTimestampManifest(strings.NewReader(out.String()), nil)
	assert.Nil(t, err)
	assert.Equal(t, *ts2, ts)
}

// TODO test that invalid manifests trigger errors
// TODO test writeManifest
