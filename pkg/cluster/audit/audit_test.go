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

	"github.com/pingcap/tiup/pkg/base52"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func auditDir() string {
	return path.Join(currentDir(), "testdata", "audit")
}

func resetDir() {
	_ = os.RemoveAll(auditDir())
	_ = os.MkdirAll(auditDir(), 0o777)
}

func readFakeStdout(f io.ReadSeeker) string {
	_, _ = f.Seek(0, 0)
	read, _ := io.ReadAll(f)
	return string(read)
}

func TestOutputAuditLog(t *testing.T) {
	dir := auditDir()
	resetDir()

	var g errgroup.Group
	for i := 0; i < 20; i++ {
		g.Go(func() error { return OutputAuditLog(dir, "", []byte("audit log")) })
	}
	err := g.Wait()
	require.NoError(t, err)

	var paths []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 20, len(paths))
}

func TestShowAuditLog(t *testing.T) {
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
		f, err := os.OpenFile(fakeStdout, os.O_CREATE|os.O_RDWR, 0o644)
		require.NoError(t, err)
		os.Stdout = f
		return f
	}

	second := int64(1604413577)
	nanoSecond := int64(1604413624836105381)

	fname := filepath.Join(dir, base52.Encode(second))
	require.NoError(t, os.WriteFile(fname, []byte("test with second"), 0o644))
	fname = filepath.Join(dir, base52.Encode(nanoSecond))
	require.NoError(t, os.WriteFile(fname, []byte("test with nanosecond"), 0o644))

	f := openStdout()
	require.NoError(t, ShowAuditList(dir))
	// tabby table size is based on column width, while time.RFC3339 maybe print out timezone like +08:00 or Z(UTC)
	// skip the first two lines
	list := strings.Join(strings.Split(readFakeStdout(f), "\n")[2:], "\n")
	require.Equal(t, fmt.Sprintf(`4F7ZTL       %s  test with second
ftmpqzww84Q  %s  test with nanosecond
`,
		time.Unix(second, 0).Format(time.RFC3339),
		time.Unix(nanoSecond/1e9, 0).Format(time.RFC3339),
	), list)
	f.Close()

	f = openStdout()
	require.NoError(t, ShowAuditLog(dir, "4F7ZTL"))
	require.Equal(t, fmt.Sprintf(`---------------------------------------
- OPERATION TIME: %s -
---------------------------------------
test with second`, time.Unix(second, 0).Format("2006-01-02T15:04:05")), readFakeStdout(f))
	f.Close()

	f = openStdout()
	require.NoError(t, ShowAuditLog(dir, "ftmpqzww84Q"))
	require.Equal(t, fmt.Sprintf(`---------------------------------------
- OPERATION TIME: %s -
---------------------------------------
test with nanosecond`, time.Unix(nanoSecond/1e9, 0).Format("2006-01-02T15:04:05")), readFakeStdout(f))
	f.Close()
}
