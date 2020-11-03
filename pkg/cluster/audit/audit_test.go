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

package audit

import (
	"os"
	"path"
	"path/filepath"
	"runtime"

	. "github.com/pingcap/check"
	"golang.org/x/sync/errgroup"
)

var _ = Suite(&testAuditSuite{})

type testAuditSuite struct{}

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func auditDir() string {
	return path.Join(currentDir(), "testdata", "audit")
}

func (s *testAuditSuite) SetUpSuite(c *C) {
	_ = os.RemoveAll(auditDir())
	_ = os.MkdirAll(auditDir(), 0777)
}

func (s *testAuditSuite) TearDownSuite(c *C) {
	_ = os.RemoveAll(auditDir())
}

func (s *testAuditSuite) TestOutputAuditLog(c *C) {
	dir := auditDir()
	var g errgroup.Group
	for i := 0; i < 20; i++ {
		g.Go(func() error { return OutputAuditLog(dir, []byte("audit log")) })
	}
	err := g.Wait()
	c.Assert(err, IsNil)

	var paths []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// simply filter the not relate files.
		paths = append(paths, path)
		return nil
	})
	c.Assert(err, IsNil)
	c.Assert(len(paths), Equals, 20)
}
