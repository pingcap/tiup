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
	os.RemoveAll(path.Join(currentDir(), "testdata", "ssh-exec"))
	os.RemoveAll(path.Join(currentDir(), "testdata", "nop-nop"))
}

func (s *TestIOUtilSuite) TearDownSuite(c *C) {
	os.RemoveAll(path.Join(currentDir(), "testdata", "parent"))
	os.RemoveAll(path.Join(currentDir(), "testdata", "ssh-exec"))
	os.RemoveAll(path.Join(currentDir(), "testdata", "nop-nop"))
}

func (s *TestIOUtilSuite) TestIsExist(c *C) {
	c.Assert(IsExist("/tmp"), IsTrue)
	c.Assert(IsExist("/tmp/"+uuid.New().String()), IsFalse)
}

func (s *TestIOUtilSuite) TestIsNotExist(c *C) {
	c.Assert(IsNotExist("/tmp"), IsFalse)
	c.Assert(IsNotExist("/tmp/"+uuid.New().String()), IsTrue)
}

func (s *TestIOUtilSuite) TestIsExecBinary(c *C) {
	c.Assert(IsExecBinary("/tmp"), IsFalse)

	e := path.Join(currentDir(), "testdata", "ssh-exec")
	f, err := os.OpenFile(e, os.O_CREATE, 0777)
	c.Assert(err, IsNil)
	defer f.Close()
	c.Assert(IsExecBinary(e), IsTrue)

	e = path.Join(currentDir(), "testdata", "nop-nop")
	f, err = os.OpenFile(e, os.O_CREATE, 0666)
	c.Assert(err, IsNil)
	defer f.Close()
	c.Assert(IsExecBinary(e), IsFalse)
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

func (s *TestIOUtilSuite) TestCopy(c *C) {
	c.Assert(Copy(path.Join(currentDir(), "testdata", "test.tar.gz"), "/tmp/not-exists/test.tar.gz"), NotNil)
	c.Assert(Copy(path.Join(currentDir(), "testdata", "test.tar.gz"), "/tmp/test.tar.gz"), IsNil)
	fi, err := os.Stat(path.Join(currentDir(), "testdata", "test.tar.gz"))
	c.Assert(err, IsNil)
	fii, err := os.Stat("/tmp/test.tar.gz")
	c.Assert(err, IsNil)
	c.Assert(fi.Mode(), Equals, fii.Mode())

	c.Assert(os.Chmod("/tmp/test.tar.gz", 0777), IsNil)
	c.Assert(Copy(path.Join(currentDir(), "testdata", "test.tar.gz"), "/tmp/test.tar.gz"), IsNil)
	fi, err = os.Stat(path.Join(currentDir(), "testdata", "test.tar.gz"))
	c.Assert(err, IsNil)
	fii, err = os.Stat("/tmp/test.tar.gz")
	c.Assert(err, IsNil)
	c.Assert(fi.Mode(), Equals, fii.Mode())
}

func (s *TestIOUtilSuite) TestIsSubDir(c *C) {
	paths := [][]string{
		{"a", "a"},
		{"../a", "../a/b"},
		{"a", "a/b"},
		{"/a", "/a/b"},
	}
	for _, p := range paths {
		c.Assert(IsSubDir(p[0], p[1]), IsTrue)
	}

	paths = [][]string{
		{"/a", "a/b"},
		{"/a/b/c", "/a/b"},
		{"/a/b", "/a/b1"},
	}
	for _, p := range paths {
		c.Assert(IsSubDir(p[0], p[1]), IsFalse)
	}
}
