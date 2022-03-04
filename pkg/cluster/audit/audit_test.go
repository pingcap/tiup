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
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/pingcap/check"
	"github.com/pingcap/tiup/pkg/base52"
	"golang.org/x/sync/errgroup"
)

func Test(t *testing.T) { TestingT(t) }

var _ = Suite(&testAuditSuite{})

type testAuditSuite struct{}

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func auditDir() string {
	return path.Join(currentDir(), "testdata", "audit")
}

func resetDir() {
	_ = os.RemoveAll(auditDir())
	_ = os.MkdirAll(auditDir(), 0777)
}

func readFakeStdout(f io.ReadSeeker) string {
	_, _ = f.Seek(0, 0)
	read, _ := io.ReadAll(f)
	return string(read)
}

func (s *testAuditSuite) SetUpSuite(c *C) {
	resetDir()
}

func (s *testAuditSuite) TearDownSuite(c *C) {
	_ = os.RemoveAll(auditDir()) // path.Join(currentDir(), "testdata"))
}

func (s *testAuditSuite) TestOutputAuditLog(c *C) {
	dir := auditDir()
	resetDir()

	var g errgroup.Group
	for i := 0; i < 20; i++ {
		g.Go(func() error { return OutputAuditLog(dir, "", []byte("audit log")) })
	}
	err := g.Wait()
	c.Assert(err, IsNil)

	var paths []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	c.Assert(err, IsNil)
	c.Assert(len(paths), Equals, 20)
}

func (s *testAuditSuite) TestShowAuditLog(c *C) {
	dir := auditDir()
	resetDir()

	originStdout := os.Stdout
	defer func() {
		os.Stdout = originStdout
	}()

	fakeStdout := path.Join(currentDir(), "fake-stdout")
	defer os.Remove(fakeStdout)

	openStdout := func() *os.File {
		_ = os.Remove(fakeStdout)
		f, err := os.OpenFile(fakeStdout, os.O_CREATE|os.O_RDWR, 0644)
		c.Assert(err, IsNil)
		os.Stdout = f
		return f
	}

	second := int64(1604413577)
	nanoSecond := int64(1604413624836105381)

	fname := filepath.Join(dir, base52.Encode(second))
	c.Assert(os.WriteFile(fname, []byte("test with second"), 0644), IsNil)
	fname = filepath.Join(dir, base52.Encode(nanoSecond))
	c.Assert(os.WriteFile(fname, []byte("test with nanosecond"), 0644), IsNil)

	f := openStdout()
	c.Assert(ShowAuditList(dir), IsNil)
	// tabby table size is based on column width, while time.RFC3339 maybe print out timezone like +08:00 or Z(UTC)
	// skip the first two lines
	list := strings.Join(strings.Split(readFakeStdout(f), "\n")[2:], "\n")
	c.Assert(list, Equals, fmt.Sprintf(`4F7ZTL       %s  test with second
ftmpqzww84Q  %s  test with nanosecond
`,
		time.Unix(second, 0).Format(time.RFC3339),
		time.Unix(nanoSecond/1e9, 0).Format(time.RFC3339),
	))
	f.Close()

	f = openStdout()
	c.Assert(ShowAuditLog(dir, "4F7ZTL"), IsNil)
	c.Assert(readFakeStdout(f), Equals, fmt.Sprintf(`---------------------------------------
- OPERATION TIME: %s -
---------------------------------------
test with second`,
		time.Unix(second, 0).Format("2006-01-02T15:04:05"),
	))

	f.Close()

	f = openStdout()
	c.Assert(ShowAuditLog(dir, "ftmpqzww84Q"), IsNil)
	c.Assert(readFakeStdout(f), Equals, fmt.Sprintf(`---------------------------------------
- OPERATION TIME: %s -
---------------------------------------
test with nanosecond`,
		time.Unix(nanoSecond/1e9, 0).Format("2006-01-02T15:04:05"),
	))
	f.Close()
}
