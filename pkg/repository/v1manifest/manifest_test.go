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

package v1manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO test that invalid manifests trigger errors
// TODO test SignAndWrite

func TestVersionItem(t *testing.T) {
	manifest := &Component{
		Platforms: map[string]map[string]VersionItem{
			"linux/amd64": {
				"v1.0.0": {Entry: "test"},
				"v1.1.1": {Entry: "test", Yanked: true},
			},
			"any/any": {
				"v1.0.0": {Entry: "test"},
			},
			// If hit this, the result of VersionItem will be nil since we don't have an entry
			"darwin/any": {
				"v1.0.0": {},
			},
			"any/arm64": {
				"v1.0.0": {},
			},
		},
	}

	assert.NotNil(t, manifest.VersionItem("linux/amd64", "v1.0.0", false))
	assert.Nil(t, manifest.VersionItem("linux/amd64", "v1.1.1", false))
	assert.NotNil(t, manifest.VersionItem("linux/amd64", "v1.1.1", true))
	assert.NotNil(t, manifest.VersionItem("windows/386", "v1.0.0", false))
	assert.NotNil(t, manifest.VersionItem("any/any", "v1.0.0", false))
	assert.Nil(t, manifest.VersionItem("darwin/any", "v1.0.0", false))
	assert.Nil(t, manifest.VersionItem("any/arm64", "v1.0.0", false))

	manifest = &Component{
		Platforms: map[string]map[string]VersionItem{
			"linux/amd64": {
				"v1.0.0": {Entry: "test"},
			},
		},
	}

	assert.NotNil(t, manifest.VersionItem("linux/amd64", "v1.0.0", false))
	assert.Nil(t, manifest.VersionItem("windows/386", "v1.0.0", false))
	assert.Nil(t, manifest.VersionItem("any/any", "v1.0.0", false))
	assert.Nil(t, manifest.VersionItem("darwin/any", "v1.0.0", false))
	assert.Nil(t, manifest.VersionItem("any/arm64", "v1.0.0", false))
}
