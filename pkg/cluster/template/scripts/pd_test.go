// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package scripts

/*
import (
	"os"
	"path"
	"strings"
	"testing"

	. "github.com/pingcap/check"
)

type pdSuite struct{}

var _ = Suite(&pdSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *pdSuite) TestScaleConfig(c *C) {
	confDir, err := os.MkdirTemp("", "tiup-*")
	c.Assert(err, IsNil)
	defer os.RemoveAll(confDir)

	conf := path.Join(confDir, "pd0.conf")
	pdScript := NewPDScript("pd", "1.1.1.1", "/home/deploy/pd-2379", "/home/pd-data", "/home/pd-log")
	pdConfig, err := pdScript.Config()
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(pdConfig), "--initial-cluster"), IsTrue)
	c.Assert(strings.Contains(string(pdConfig), "--join"), IsFalse)

	err = pdScript.ConfigToFile(conf)
	c.Assert(err, IsNil)
	content, err := os.ReadFile(conf)
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(content), "--initial-cluster"), IsTrue)
	c.Assert(strings.Contains(string(content), "--join"), IsFalse)

	scScript := NewPDScaleScript(pdScript)
	scConfig, err := scScript.Config()
	c.Assert(err, IsNil)
	c.Assert(pdConfig, Not(DeepEquals), scConfig)
	c.Assert(strings.Contains(string(scConfig), "--initial-cluster"), IsFalse)
	c.Assert(strings.Contains(string(scConfig), "--join"), IsTrue)

	err = scScript.ConfigToFile(conf)
	c.Assert(err, IsNil)
	content, err = os.ReadFile(conf)
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(content), "--initial-cluster"), IsFalse)
	c.Assert(strings.Contains(string(content), "--join"), IsTrue)
}
*/
