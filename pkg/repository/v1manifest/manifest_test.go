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
			"linux/amd64": map[string]VersionItem{
				"v1.0.0": VersionItem{Entry: "test"},
			},
			"any/any": map[string]VersionItem{
				"v1.0.0": VersionItem{Entry: "test"},
			},
		},
	}

	assert.NotNil(t, manifest.VersionItem("linux/amd64", "v1.0.0"))
	assert.NotNil(t, manifest.VersionItem("windows/386", "v1.0.0"))

	manifest = &Component{
		Platforms: map[string]map[string]VersionItem{
			"linux/amd64": map[string]VersionItem{
				"v1.0.0": VersionItem{Entry: "test"},
			},
		},
	}

	assert.NotNil(t, manifest.VersionItem("linux/amd64", "v1.0.0"))
	assert.Nil(t, manifest.VersionItem("windows/386", "v1.0.0"))
}
