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
	"runtime"
	"testing"

	. "github.com/pingcap/check"
	"github.com/pingcap/failpoint"
)

type metaSuite struct{}

var _ = Suite(&metaSuite{})

func TestMeta(t *testing.T) {
	TestingT(t)
}

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func (s *metaSuite) TestRepository(c *C) {
	testDir := filepath.Join(currentDir(), "testdata")
	repo := NewRepository(NewMirror(testDir), RepositoryOptions{})
	comps, err := repo.Manifest()
	c.Assert(err, IsNil)

	expected := &ComponentManifest{
		Description: "TiDB components list",
		Modified:    "2020-02-26T15:20:35+08:00",
		Components: []ComponentInfo{
			{Name: "test1", Desc: "desc1", Platforms: []string{"darwin/amd64", "linux/amd64"}},
			{Name: "test2", Desc: "desc2", Platforms: []string{"darwin/amd64", "linux/amd64"}},
			{Name: "test3", Desc: "desc3", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
		},
	}
	c.Assert(comps, DeepEquals, expected)

	cases := []struct {
		comp string
		vers []VersionInfo
	}{
		{
			comp: "test1",
			vers: []VersionInfo{
				{Version: "v1.1.1", Date: "2020-02-27 10:10:10", Entry: "test1.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
				{Version: "v1.1.2", Date: "2020-02-27 10:10:10", Entry: "test1.bin", Platforms: []string{"darwin/amd64", "linux/amd64"}},
				{Version: "v1.1.3", Date: "2020-02-27 10:10:10", Entry: "test1.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
			},
		},
		{
			comp: "test2",
			vers: []VersionInfo{
				{Version: "v2.1.1", Date: "2020-02-27 10:20:10", Entry: "test2.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
				{Version: "v2.1.2", Date: "2020-02-27 10:20:10", Entry: "test2.bin", Platforms: []string{"darwin/amd64", "linux/amd64"}},
				{Version: "v2.1.3", Date: "2020-02-27 10:20:10", Entry: "test2.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
			},
		},
		{
			comp: "test3",
			vers: []VersionInfo{
				{Version: "v3.1.1", Date: "2020-02-27 10:30:10", Entry: "test3.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
				{Version: "v3.1.2", Date: "2020-02-27 10:30:10", Entry: "test3.bin", Platforms: []string{"darwin/amd64", "linux/amd64"}},
				{Version: "v3.1.3", Date: "2020-02-27 10:30:10", Entry: "test3.bin", Platforms: []string{"darwin/amd64", "linux/x84_64"}},
			},
		},
	}

	for _, cas := range cases {
		vers, err := repo.ComponentVersions(cas.comp)
		c.Assert(err, IsNil)
		c.Assert(vers.Description, Equals, cas.comp)
		c.Assert(vers.Modified, Equals, "2020-02-26T15:20:35+08:00")
		c.Assert(vers.Versions, DeepEquals, cas.vers)
	}

	tmpDir := filepath.Join(currentDir(), "tmp-profile")
	fpName := "github.com/pingcap-incubator/tiup/pkg/meta/MockProfileDir"
	fpExpr := `return("` + tmpDir + `")`
	c.Assert(failpoint.Enable(fpName, fpExpr), IsNil)
	defer func() { c.Assert(failpoint.Disable(fpName), IsNil) }()
	defer os.RemoveAll(tmpDir)

	err = repo.DownloadComponent(tmpDir, "test1:v1.1.1")
	c.Assert(err, IsNil, Commentf("error: %+v", err))

	exp, err := ioutil.ReadFile(filepath.Join(testDir, "test1.bin"))
	got, err := ioutil.ReadFile(filepath.Join(tmpDir, "test1", "v1.1.1", "test1.bin"))
	c.Assert(err, IsNil)

	c.Assert(got, DeepEquals, exp)
}
