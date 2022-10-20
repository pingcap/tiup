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

package localdata

import (
	"os"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/utils"
)

var _ = check.Suite(&profileTestSuite{})

type profileTestSuite struct{}

func TestProfile(t *testing.T) {
	check.TestingT(t)
}

func (s *profileTestSuite) TestResetMirror(c *check.C) {
	uuid := uuid.New().String()
	root := path.Join("/tmp", uuid)
	_ = os.Mkdir(root, 0755)
	_ = os.Mkdir(path.Join(root, "bin"), 0755)
	defer os.RemoveAll(root)

	cfg, _ := InitConfig(root)
	profile := NewProfile(root, "todo", cfg)

	c.Assert(profile.ResetMirror("https://tiup-mirrors.pingcap.com", ""), check.IsNil)
	c.Assert(profile.ResetMirror("https://example.com", ""), check.NotNil)
	c.Assert(profile.ResetMirror("https://example.com", "https://tiup-mirrors.pingcap.com/root.json"), check.IsNil)

	c.Assert(utils.Copy(path.Join(root, "bin"), path.Join(root, "mock-mirror")), check.IsNil)

	c.Assert(profile.ResetMirror(path.Join(root, "mock-mirror"), ""), check.IsNil)
	c.Assert(profile.ResetMirror(root, ""), check.NotNil)
	c.Assert(profile.ResetMirror(root, path.Join(root, "mock-mirror", "root.json")), check.IsNil)
}
