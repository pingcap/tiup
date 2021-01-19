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
	"testing"

	. "github.com/pingcap/check"
)

type repositorySuite struct{}

var _ = Suite(&repositorySuite{})

func TestRepository(t *testing.T) {
	TestingT(t)
}

type mockFileSource struct {
	jsonFn func(string, interface{})
	tarFn  func(string, string, bool)
}

func (fs *mockFileSource) open() error {
	return nil
}

func (fs *mockFileSource) close() error {
	return nil
}

func (fs *mockFileSource) downloadJSON(resource string, result interface{}) error {
	if fs.jsonFn != nil {
		fs.jsonFn(resource, result)
	}
	return nil
}

func (fs *mockFileSource) downloadTarFile(targetDir, resName string, expand bool) error {
	if fs.tarFn != nil {
		fs.tarFn(targetDir, resName, expand)
	}
	return nil
}

// FIXME test DownloadComponent sad paths

func (s *repositorySuite) TestDownloadTiup(c *C) {
	fs := &mockFileSource{}
	fs.tarFn = func(targetDir, resName string, expand bool) {
		c.Assert(expand, IsTrue)
		c.Assert(targetDir, Equals, "foo")
		c.Assert(resName, Equals, "tiup-baz-bar")
	}
	repo := Repository{fileSource: fs, Options: Options{GOARCH: "bar", GOOS: "baz"}}
	err := repo.DownloadTiUP("foo")
	c.Assert(err, IsNil, Commentf("error: %+v", err))
}
