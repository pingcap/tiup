package file

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pingcap/check"
)

func Test(t *testing.T) { check.TestingT(t) }

type fileSuite struct{}

var _ = check.Suite(&fileSuite{})

func (s *fileSuite) TestSaveFileWithBackup(c *check.C) {
	dir := c.MkDir()
	name := "meta.yaml"

	for i := 0; i < 10; i++ {
		err := SaveFileWithBackup(filepath.Join(dir, name), []byte(strconv.Itoa(i)), "")
		c.Assert(err, check.IsNil)
	}

	// Verify the saved files.
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// simply filter the not relate files.
		if strings.Contains(path, "meta") {
			paths = append(paths, path)
		}
		return nil
	})

	c.Assert(err, check.IsNil)
	c.Assert(len(paths), check.Equals, 10)

	sort.Strings(paths)
	for i, path := range paths {
		data, err := os.ReadFile(path)
		c.Assert(err, check.IsNil)
		c.Assert(string(data), check.Equals, strconv.Itoa(i))
	}

	// test with specify backup dir
	dir = c.MkDir()
	backupDir := c.MkDir()
	for i := 0; i < 10; i++ {
		err := SaveFileWithBackup(filepath.Join(dir, name), []byte(strconv.Itoa(i)), backupDir)
		c.Assert(err, check.IsNil)
	}
	// Verify the saved files in backupDir.
	paths = nil
	err = filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		// simply filter the not relate files.
		if strings.Contains(path, "meta") {
			paths = append(paths, path)
		}
		return nil
	})
	c.Assert(err, check.IsNil)
	c.Assert(len(paths), check.Equals, 9)

	sort.Strings(paths)
	for i, path := range paths {
		data, err := os.ReadFile(path)
		c.Assert(err, check.IsNil)
		c.Assert(string(data), check.Equals, strconv.Itoa(i))
	}

	// Verify the latest saved file.
	data, err := os.ReadFile(filepath.Join(dir, name))
	c.Assert(err, check.IsNil)
	c.Assert(string(data), check.Equals, "9")
}

func (s *fileSuite) TestConcurrentSaveFileWithBackup(c *check.C) {
	dir := c.MkDir()
	name := "meta.yaml"
	data := []byte("concurrent-save-file-with-backup")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(rand.Intn(100)+4) * time.Millisecond)
			err := SaveFileWithBackup(filepath.Join(dir, name), data, "")
			c.Assert(err, check.IsNil)
		}()
	}

	wg.Wait()

	// Verify the saved files.
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// simply filter the not relate files.
		if strings.Contains(path, "meta") {
			paths = append(paths, path)
		}
		return nil
	})

	c.Assert(err, check.IsNil)
	c.Assert(len(paths), check.Equals, 10)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		c.Assert(err, check.IsNil)
		c.Assert(len(body), check.Equals, len(data))
		c.Assert(bytes.Equal(body, data), check.IsTrue)
	}
}
