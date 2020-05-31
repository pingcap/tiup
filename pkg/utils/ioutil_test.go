package utils

import (
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/google/uuid"
	. "github.com/pingcap/check"
)

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

var _ = Suite(&TestIOUtilSuite{})

type TestIOUtilSuite struct{}

func (s *TestIOUtilSuite) TestIOUtil(c *C) {}

func (s *TestIOUtilSuite) SetUpSuite(c *C) {
	os.RemoveAll(path.Join(currentDir(), "testdata", "parent"))
}

func (s *TestIOUtilSuite) TearDownSuite(c *C) {
	os.RemoveAll(path.Join(currentDir(), "testdata", "parent"))
}

func (s *TestIOUtilSuite) TestIsExist(c *C) {
	c.Assert(IsExist("/tmp"), IsTrue)
	c.Assert(IsExist("/tmp/"+uuid.New().String()), IsFalse)
}

func (s *TestIOUtilSuite) TestIsNotExist(c *C) {
	c.Assert(IsNotExist("/tmp"), IsFalse)
	c.Assert(IsNotExist("/tmp/"+uuid.New().String()), IsTrue)
}

func (s *TestIOUtilSuite) TestUntar(c *C) {
	c.Assert(IsNotExist(path.Join(currentDir(), "testdata", "parent")), IsTrue)
	f, err := os.Open(path.Join(currentDir(), "testdata", "test.tar.gz"))
	c.Assert(err, IsNil)
	defer f.Close()
	err = Untar(f, path.Join(currentDir(), "testdata"))
	c.Assert(err, IsNil)
	c.Assert(IsExist(path.Join(currentDir(), "testdata", "parent", "child", "content")), IsTrue)
}
