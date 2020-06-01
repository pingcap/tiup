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
	"strings"
	"testing"

	. "github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
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

var expManifest = &v0manifest.ComponentManifest{
	Description: "TiDB components list",
	Modified:    "2020-02-26T15:20:35+08:00",
	Components: []v0manifest.ComponentInfo{
		{Name: "test1", Desc: "desc1", Platforms: []string{"darwin/amd64", "linux/amd64"}},
		{Name: "test2", Desc: "desc2", Platforms: []string{"darwin/amd64", "linux/amd64"}},
		{Name: "test3", Desc: "desc3", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
	},
}

var cases = map[string][]v0manifest.VersionInfo{
	"test1": {
		{Version: "v1.1.1", Date: "2020-02-27T10:10:10+08:00", Entry: "test1.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
		{Version: "v1.1.2", Date: "2020-02-27T10:10:10+08:00", Entry: "test1.bin", Platforms: []string{"darwin/amd64", "linux/amd64"}},
		{Version: "v1.1.3", Date: "2020-02-27T10:10:10+08:00", Entry: "test1.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
	},
	"test2": {
		{Version: "v2.1.1", Date: "2020-02-27T10:20:10+08:00", Entry: "test2.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
		{Version: "v2.1.2", Date: "2020-02-27T10:20:10+08:00", Entry: "test2.bin", Platforms: []string{"darwin/amd64", "linux/amd64"}},
		{Version: "v2.1.3", Date: "2020-02-27T10:20:10+08:00", Entry: "test2.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
	},
	"test3": {
		{Version: "v3.1.1", Date: "2020-02-27T10:30:10+08:00", Entry: "test3.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
		{Version: "v3.1.2", Date: "2020-02-27T10:30:10+08:00", Entry: "test3.bin", Platforms: []string{"darwin/amd64", "linux/amd64"}},
		{Version: "v3.1.3", Date: "2020-02-27T10:30:10+08:00", Entry: "test3.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
	},
}

func (s *repositorySuite) TestManifest(c *C) {
	fs := &mockFileSource{}
	fs.jsonFn = func(resource string, result interface{}) {
		c.Assert(resource, Equals, ManifestFileName)
		manifest, ok := result.(*v0manifest.ComponentManifest)
		c.Assert(ok, IsTrue)
		*manifest = *expManifest
	}
	repo := Repository{fileSource: fs}
	manifest, err := repo.Manifest()
	c.Assert(err, IsNil)
	c.Assert(manifest, DeepEquals, expManifest)
	c.Assert(repo.Mirror(), IsNil)
}

var handleVersManifests = func(resource string, result interface{}) {
	if !strings.HasPrefix(resource, "tiup-component-") || !strings.HasSuffix(resource, ".index") {
		panic("bad resource string")
	}
	name := resource[len("tiup-component-") : len(resource)-len(".index")]
	m, _ := result.(*v0manifest.VersionManifest)
	*m = v0manifest.VersionManifest{
		Description: name,
		Modified:    "2020-02-26T15:20:35+08:00",
		Versions:    cases[name],
	}
}

func (s *repositorySuite) TestVerManifest(c *C) {
	fs := &mockFileSource{}
	fs.jsonFn = handleVersManifests
	repo := Repository{fileSource: fs}
	for comp, exVers := range cases {
		vers, err := repo.ComponentVersions(comp)
		c.Assert(err, IsNil)
		c.Assert(vers.Description, Equals, comp)
		c.Assert(vers.Modified, Equals, "2020-02-26T15:20:35+08:00")
		c.Assert(vers.Versions, DeepEquals, exVers)
	}
}

func (s *repositorySuite) TestDownload(c *C) {
	fs := &mockFileSource{}
	fs.jsonFn = handleVersManifests
	fs.tarFn = func(targetDir, resName string, expand bool) {
		c.Assert(expand, IsTrue)
		c.Assert(targetDir, Equals, "foo/test1/v1.1.1")
		c.Assert(resName, Equals, "test1-v1.1.1-baz-bar")
	}
	repo := Repository{fileSource: fs, Options: Options{GOARCH: "bar", GOOS: "baz"}}
	err := repo.DownloadComponent("foo", "test1", "v1.1.1")
	c.Assert(err, IsNil, Commentf("error: %+v", err))
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
	err := repo.DownloadTiup("foo")
	c.Assert(err, IsNil, Commentf("error: %+v", err))
}
