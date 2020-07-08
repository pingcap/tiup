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

package meta

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpec(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-*")
	assert.Nil(t, err)

	spec := NewSpec(dir)
	names, err := spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 0)

	// Should ignore directory without meta file.
	err = os.Mkdir(filepath.Join(dir, "dummy"), 0755)
	assert.Nil(t, err)
	names, err = spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 0)

	exist, err := spec.Exist("dummy")
	assert.Nil(t, err)
	assert.False(t, exist)

	type Meta struct {
		A string
		B int
	}
	var meta1 = &Meta{
		A: "a",
		B: 1,
	}
	var meta2 = &Meta{
		A: "b",
		B: 2,
	}

	err = spec.SaveMeta("name1", meta1)
	assert.Nil(t, err)

	err = spec.SaveMeta("name2", meta2)
	assert.Nil(t, err)

	getMeta := new(Meta)
	err = spec.Metadata("name1", getMeta)
	assert.Nil(t, err)
	assert.Equal(t, meta1, getMeta)

	err = spec.Metadata("name2", getMeta)
	assert.Nil(t, err)
	assert.Equal(t, meta2, getMeta)

	names, err = spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 2)
	sort.Strings(names)
	assert.Equal(t, "name1", names[0])
	assert.Equal(t, "name2", names[1])

	exist, err = spec.Exist("name1")
	assert.Nil(t, err)
	assert.True(t, exist)

	exist, err = spec.Exist("name2")
	assert.Nil(t, err)
	assert.True(t, exist)

	// remove name1 and check again.
	err = spec.Remove("name1")
	assert.Nil(t, err)
	exist, err = spec.Exist("name1")
	assert.Nil(t, err)
	assert.False(t, exist)

	// remove a not exist cluster should be fine
	err = spec.Remove("name1")
	assert.Nil(t, err)
}
