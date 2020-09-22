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

package cmd

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/repository"
)

func TestCMD(t *testing.T) {
	TestingT(t)
}

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

var _ = Suite(&testCmdSuite{})

type testCmdSuite struct {
	mirror  repository.Mirror
	testDir string
}

func (s *testCmdSuite) SetUpSuite(c *C) {
	s.testDir = filepath.Join(currentDir(), "testdata")
	s.mirror = repository.NewMirror(s.testDir, repository.MirrorOptions{})
	_ = s.mirror.Open()
}

/*
func (s *testCmdSuite) newEnv(c *C) *meta.Environment {
	repo, err := repository.NewRepository(s.mirror, repository.Options{})
	c.Assert(err, IsNil)

	c.Assert(os.RemoveAll(path.Join(s.testDir, "profile")), IsNil)
	c.Assert(os.MkdirAll(path.Join(s.testDir, "profile"), 0755), IsNil)
	profile := localdata.NewProfile(path.Join(s.testDir, "profile"))

	return meta.NewV0(profile, repo)
}
*/

func (s *testCmdSuite) TearDownSuite(c *C) {
	s.mirror.Close()
	c.Assert(os.RemoveAll(path.Join(s.testDir, "profile")), IsNil)
}

// TODO: add back these tests after we have mock test data
// For now, disable it temporary
/*
func (s *testCmdSuite) TestInstall(c *C) {
	cmd := newInstallCmd(s.newEnv(c))

	c.Assert(utils.IsNotExist(path.Join(s.testDir, "profile", "components", "test")), IsTrue)
	c.Assert(cmd.RunE(cmd, []string{"test"}), IsNil)
	c.Assert(utils.IsExist(path.Join(s.testDir, "profile", "components", "test")), IsTrue)
}

func (s *testCmdSuite) TestListComponent(c *C) {
	result, err := showComponentList(s.newEnv(c), false, false)
	c.Assert(err, IsNil)
	c.Assert(len(result.cmpTable), Greater, 1)
	c.Assert(result.cmpTable[1][0], Equals, "test")
}

func (s *testCmdSuite) TestListVersion(c *C) {
	result, err := showComponentVersions(s.newEnv(c), "test", false, false)
	c.Assert(err, IsNil)
	result.print()
	c.Assert(len(result.cmpTable), Greater, 1)
	for idx := 1; idx < len(result.cmpTable); idx++ {
		success := false
		if strings.HasPrefix(result.cmpTable[idx][0], "v") || result.cmpTable[idx][0] == "nightly" {
			success = true
		}
		c.Assert(success, IsTrue)
	}
}
*/
