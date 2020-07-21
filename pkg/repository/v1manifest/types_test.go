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

	"github.com/alecthomas/assert"
)

func TestComponentList(t *testing.T) {
	manifest := &Index{
		Components: map[string]ComponentItem{
			"comp1": {},
			"comp2": {Yanked: true},
		},
	}

	list := manifest.ComponentList()
	assert.Equal(t, len(list), 1)
	_, ok := list["comp1"]
	assert.True(t, ok)

	list = manifest.ComponentListWithYanked()
	assert.Equal(t, len(list), 2)
	_, ok = list["comp1"]
	assert.True(t, ok)
	_, ok = list["comp2"]
	assert.True(t, ok)
}

func TestVersionList(t *testing.T) {
	manifest := &Component{
		Platforms: map[string]map[string]VersionItem{
			"linux/amd64": {
				"v1.0.0": {Entry: "test"},
				"v1.1.1": {Entry: "test", Yanked: true},
			},
			"any/any": {
				"v1.0.0": {Entry: "test"},
				"v1.1.1": {Entry: "test", Yanked: true},
			},
		},
	}

	versions := manifest.VersionList("linux/amd64")
	assert.Equal(t, len(versions), 1)
	_, ok := versions["v1.0.0"]
	assert.True(t, ok)

	versions = manifest.VersionListWithYanked("linux/amd64")
	assert.Equal(t, len(versions), 2)
	_, ok = versions["v1.0.0"]
	assert.True(t, ok)
	_, ok = versions["v1.1.1"]
	assert.True(t, ok)

	versions = manifest.VersionList("windows/amd64")
	assert.Equal(t, len(versions), 1)
	_, ok = versions["v1.0.0"]
	assert.True(t, ok)

	manifest = &Component{
		Platforms: map[string]map[string]VersionItem{
			"linux/amd64": {
				"v1.0.0": {Entry: "test"},
				"v1.1.1": {Entry: "test", Yanked: true},
			},
		},
	}

	versions = manifest.VersionList("windows/amd64")
	assert.Equal(t, len(versions), 0)
}
