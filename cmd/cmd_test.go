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
	"strings"
	"testing"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/mock"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	. "github.com/pingcap/check"
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
	s.mirror = repository.NewMirror(s.testDir)
	c.Assert(s.mirror.Open(), IsNil)
	meta.SetRepository(repository.NewRepository(s.mirror, repository.Options{}))
	c.Assert(os.RemoveAll(path.Join(s.testDir, "profile")), IsNil)
	c.Assert(os.MkdirAll(path.Join(s.testDir, "profile"), 0755), IsNil)
	meta.SetProfile(localdata.NewProfile(path.Join(s.testDir, "profile")))
}

func (s *testCmdSuite) TearDownSuite(c *C) {
	s.mirror.Close()
	c.Assert(os.RemoveAll(path.Join(s.testDir, "profile")), IsNil)
}

func (s *testCmdSuite) TestInstall(c *C) {
	cmd := newInstallCmd()

	c.Assert(utils.IsNotExist(path.Join(s.testDir, "profile", "components", "test")), IsTrue)
	c.Assert(cmd.RunE(cmd, []string{"test"}), IsNil)
	c.Assert(utils.IsExist(path.Join(s.testDir, "profile", "components", "test")), IsTrue)
}

func (s *testCmdSuite) TestListComponent(c *C) {
	cmd := newListCmd()
	defer mock.With("PrintTable", func(rows [][]string, header bool) {
		c.Assert(header, IsTrue)
		c.Assert(len(rows), Greater, 1)
		c.Assert(rows[1][0], Equals, "test")
	})()

	c.Assert(cmd.RunE(cmd, []string{}), IsNil)
}

func (s *testCmdSuite) TestListVersion(c *C) {
	cmd := newListCmd()
	defer mock.With("PrintTable", func(rows [][]string, header bool) {
		c.Assert(header, IsTrue)
		c.Assert(len(rows), Greater, 1)
		for idx := 1; idx < len(rows); idx++ {
			success := false
			if strings.HasPrefix(rows[idx][0], "v") || rows[idx][0] == "nightly" {
				success = true
			}
			c.Assert(success, IsTrue)
		}
	})()

	c.Assert(cmd.RunE(cmd, []string{"test"}), IsNil)
}
