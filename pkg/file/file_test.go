package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

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
		data, err := ioutil.ReadFile(path)
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
		data, err := ioutil.ReadFile(path)
		c.Assert(err, check.IsNil)
		c.Assert(string(data), check.Equals, strconv.Itoa(i))
	}

	// Verify the latest saved file.
	data, err := ioutil.ReadFile(filepath.Join(dir, name))
	c.Assert(err, check.IsNil)
	c.Assert(string(data), check.Equals, "9")
}
