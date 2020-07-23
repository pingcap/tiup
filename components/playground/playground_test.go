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
	"os/user"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/tj/assert"
)

func TestPlaygroundAbsDir(t *testing.T) {
	err := os.Setenv(localdata.EnvNameWorkDir, "/testing")
	assert.Nil(t, err)

	a, err := getAbsolutePath("./a")
	assert.Nil(t, err)
	assert.Equal(t, "/testing/a", a)

	b, err := getAbsolutePath("../b")
	assert.Nil(t, err)
	assert.Equal(t, "/b", b)

	u, err := user.Current()
	assert.Nil(t, err)
	c, err := getAbsolutePath("~/c/d/e")
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(u.HomeDir, "c/d/e"), c)
}
