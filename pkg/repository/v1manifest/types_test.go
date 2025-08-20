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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentList(t *testing.T) {
	manifest := &Index{
		Components: map[string]ComponentItem{
			"comp1": {},
			"comp2": {Yanked: true},
		},
	}

	list := manifest.ComponentList()
	require.Equal(t, len(list), 1)
	_, ok := list["comp1"]
	require.True(t, ok)

	list = manifest.ComponentListWithYanked()
	require.Equal(t, len(list), 2)
	_, ok = list["comp1"]
	require.True(t, ok)
	_, ok = list["comp2"]
	require.True(t, ok)
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
	require.Equal(t, len(versions), 1)
	_, ok := versions["v1.0.0"]
	require.True(t, ok)

	versions = manifest.VersionListWithYanked("linux/amd64")
	require.Equal(t, len(versions), 2)
	_, ok = versions["v1.0.0"]
	require.True(t, ok)
	_, ok = versions["v1.1.1"]
	require.True(t, ok)

	versions = manifest.VersionList("windows/amd64")
	require.Equal(t, len(versions), 1)
	_, ok = versions["v1.0.0"]
	require.True(t, ok)

	manifest = &Component{
		Platforms: map[string]map[string]VersionItem{
			"linux/amd64": {
				"v1.0.0": {Entry: "test"},
				"v1.1.1": {Entry: "test", Yanked: true},
			},
		},
	}

	versions = manifest.VersionList("windows/amd64")
	require.Equal(t, len(versions), 0)
}

func TestLoadManifestError(t *testing.T) {
	err0 := &LoadManifestError{
		manifest: "root.json",
		err:      fmt.Errorf("dummy error"),
	}
	// identical errors are equal
	require.True(t, errors.Is(err0, err0))
	require.True(t, errors.Is(ErrLoadManifest, ErrLoadManifest))
	require.True(t, errors.Is(ErrLoadManifest, &LoadManifestError{}))
	require.True(t, errors.Is(&LoadManifestError{}, ErrLoadManifest))
	// not equal for different error types
	require.False(t, errors.Is(err0, errors.New("")))
	// default Value matches any error
	require.True(t, errors.Is(err0, ErrLoadManifest))
	// error with values are not matching default ones
	require.False(t, errors.Is(ErrLoadManifest, err0))

	err1 := &LoadManifestError{
		manifest: "root.json",
		err:      fmt.Errorf("dummy error 2"),
	}
	require.True(t, errors.Is(err1, ErrLoadManifest))
	// errors with different errors are different
	require.False(t, errors.Is(err0, err1))
	require.False(t, errors.Is(err1, err0))

	err2 := &LoadManifestError{
		manifest: "root.json",
	}
	// nil errors can be match with any error, but not vise vera
	require.True(t, errors.Is(err1, err2))
	require.False(t, errors.Is(err2, err1))
}
