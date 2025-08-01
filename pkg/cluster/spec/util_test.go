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

package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAbs(t *testing.T) {
	var path string
	path = Abs(" foo", "")
	require.Equal(t, "/home/foo", path)
	path = Abs("foo ", " ")
	require.Equal(t, "/home/foo", path)
	path = Abs("foo", "bar")
	require.Equal(t, "/home/foo/bar", path)
	path = Abs("foo", " bar")
	require.Equal(t, "/home/foo/bar", path)
	path = Abs("foo", "bar ")
	require.Equal(t, "/home/foo/bar", path)
	path = Abs("foo", " bar ")
	require.Equal(t, "/home/foo/bar", path)
	path = Abs("foo", "/bar")
	require.Equal(t, "/bar", path)
	path = Abs("foo", " /bar")
	require.Equal(t, "/bar", path)
	path = Abs("foo", "/bar ")
	require.Equal(t, "/bar", path)
	path = Abs("foo", " /bar ")
	require.Equal(t, "/bar", path)
}

func TestMultiDirAbs(t *testing.T) {
	paths := MultiDirAbs("tidb", "")
	require.Equal(t, 0, len(paths))

	paths = MultiDirAbs("tidb", " ")
	require.Equal(t, 0, len(paths))

	paths = MultiDirAbs("tidb", "a ")
	require.Equal(t, 1, len(paths))
	require.Equal(t, "/home/tidb/a", paths[0])

	paths = MultiDirAbs("tidb", "a , /tmp/b")
	require.Equal(t, 2, len(paths))
	require.Equal(t, "/home/tidb/a", paths[0])
	require.Equal(t, "/tmp/b", paths[1])
}
