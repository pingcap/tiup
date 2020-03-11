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
	"strings"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/mock"
	. "github.com/pingcap/check"
)

var _ = Suite(&TestListSuite{})

type TestListSuite struct {
	mirror  meta.Mirror
	testDir string
}

func (s *TestListSuite) SetUpSuite(c *C) {
	s.testDir = filepath.Join(currentDir(), "testdata")
	s.mirror = meta.NewMirror(s.testDir)
	c.Assert(s.mirror.Open(), IsNil)
	repository = meta.NewRepository(s.mirror, meta.RepositoryOptions{})
	os.RemoveAll(path.Join(s.testDir, "profile"))
	os.MkdirAll(path.Join(s.testDir, "profile"), 0755)
	profile = localdata.NewProfile(path.Join(s.testDir, "profile"))
}

func (s *TestListSuite) TearDownSuite(c *C) {
	s.mirror.Close()
	os.RemoveAll(path.Join(s.testDir, "profile"))
}

func (s *TestListSuite) TestListComponent(c *C) {
	cmd := newListCmd()
	defer mock.With("PrintTable", func(rows [][]string, header bool) {
		c.Assert(header, IsTrue)
		c.Assert(len(rows), Greater, 1)
		c.Assert(rows[1][0], Equals, "test")
	})()

	c.Assert(cmd.RunE(cmd, []string{}), IsNil)
}

func (s *TestListSuite) TestListVersion(c *C) {
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
