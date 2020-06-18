package clusterutil

import (
	"github.com/pingcap/check"
)

type utilSuite struct{}

var _ = check.Suite(&utilSuite{})

func (s *utilSuite) TestMultiDirAbs(c *check.C) {
	paths := MultiDirAbs("tidb", "")
	c.Assert(len(paths), check.Equals, 0)

	paths = MultiDirAbs("tidb", " ")
	c.Assert(len(paths), check.Equals, 0)

	paths = MultiDirAbs("tidb", "a ")
	c.Assert(len(paths), check.Equals, 1)
	c.Assert(paths[0], check.Equals, "/home/tidb/a")

	paths = MultiDirAbs("tidb", "a , /tmp/b")
	c.Assert(len(paths), check.Equals, 2)
	c.Assert(paths[0], check.Equals, "/home/tidb/a")
	c.Assert(paths[1], check.Equals, "/tmp/b")
}
