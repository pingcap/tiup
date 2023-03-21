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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestReplaceDatasource(t *testing.T) {
	origin := `
a: ${DS_1-CLUSTER}
b: test-cluster
c: Test-Cluster
d: ${DS_LIGHTNING}
	`

	dir, err := os.MkdirTemp("", "play_replace_test_*")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	fname := filepath.Join(dir, "a.json")
	err = os.WriteFile(fname, []byte(origin), 0644)
	assert.Nil(t, err)

	name := "myname"
	err = replaceDatasource(dir, name)
	assert.Nil(t, err)

	data, err := os.ReadFile(fname)
	assert.Nil(t, err)
	replaced := string(data)

	n := strings.Count(replaced, name)
	assert.Equal(t, 4, n, "replaced: %s", replaced)
}
