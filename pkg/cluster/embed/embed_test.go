package embed

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pingcap/check"
)

func Test(t *testing.T) { check.TestingT(t) }

type embedSuite struct{}

var _ = check.Suite(&embedSuite{})

func getAllFilePaths(dir string) (paths []string, err error) {
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if path == dir {
			return nil
		}
		if info.IsDir() {
			subPaths, err := getAllFilePaths(path)
			if err != nil {
				return err
			}
			paths = append(paths, subPaths...)
		} else {
			paths = append(paths, path)
		}

		return nil
	})

	return
}

// Test can read all file in /templates
// If files in /templates changed, you may need run `make pkger` to regenerate the autogen_pkger.go
func (s *embedSuite) TestCanReadTemplates(c *check.C) {
	root, err := filepath.Abs("../../../")
	c.Assert(err, check.IsNil)

	paths, err := getAllFilePaths(filepath.Join(root, "templates"))
	c.Assert(err, check.IsNil)
	c.Assert(len(paths), check.Greater, 0)

	for _, path := range paths {
		embedPath := strings.TrimPrefix(path, root)
		c.Log("check file: ", path, " ", embedPath)

		data, err := ioutil.ReadFile(path)
		c.Assert(err, check.IsNil)

		embedData, err := ReadFile(embedPath)
		c.Assert(err, check.IsNil)

		c.Assert(embedData, check.BytesEquals, data)
	}
}
