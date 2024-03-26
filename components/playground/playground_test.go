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

	"github.com/stretchr/testify/assert"
)

func TestPlaygroundAbsDir(t *testing.T) {
	err := os.Chdir("/usr")
	assert.Nil(t, err)

	a, err := getAbsolutePath("./a")
	assert.Nil(t, err)
	assert.Equal(t, "/usr/a", a)

	b, err := getAbsolutePath("../b")
	assert.Nil(t, err)
	assert.Equal(t, "/b", b)

	u, err := user.Current()
	assert.Nil(t, err)
	c, err := getAbsolutePath("~/c/d/e")
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(u.HomeDir, "c/d/e"), c)
}

func TestParseMysqlCommand(t *testing.T) {
	cases := []struct {
		version string
		vMaj    int
		vMin    int
		vPatch  int
		err     bool
	}{
		{
			"mysql  Ver 8.2.0 for Linux on x86_64 (MySQL Community Server - GPL)",
			8,
			2,
			0,
			false,
		},
		{
			"mysql  Ver 8.0.34 for Linux on x86_64 (MySQL Community Server - GPL)",
			8,
			0,
			34,
			false,
		},
		{
			"mysql  Ver 8.0.34-foobar for Linux on x86_64 (MySQL Community Server - GPL)",
			8,
			0,
			34,
			false,
		},
		{
			"foobar",
			0,
			0,
			0,
			true,
		},
		{
			"mysql  Ver 14.14 Distrib 5.7.36, for linux-glibc2.12 (x86_64) using  EditLine wrapper",
			5,
			7,
			36,
			false,
		},
		{
			"mysql  Ver 15.1 Distrib 10.3.37-MariaDB, for Linux (x86_64) using readline 5.1",
			10,
			3,
			37,
			false,
		},
		{
			"/bin/mysql from 11.2.2-MariaDB, client 15.2 for linux-systemd (x86_64) using readline 5.1",
			11,
			2,
			2,
			false,
		},
	}

	for _, tc := range cases {
		vMaj, vMin, vPatch, err := parseMysqlVersion(tc.version)
		if tc.err {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
		}
		assert.Equal(t, vMaj, tc.vMaj)
		assert.Equal(t, vMin, tc.vMin)
		assert.Equal(t, vPatch, tc.vPatch)
	}
}
