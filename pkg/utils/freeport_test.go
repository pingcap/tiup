package utils

import (
	. "github.com/pingcap/check"
)

var _ = Suite(&TestFreePortSuite{})

type TestFreePortSuite struct{}

func (s *TestFreePortSuite) TestGetFreePort(c *C) {
	expected := 22334
	port, err := getFreePort("127.0.0.1", expected)
	c.Assert(err, IsNil)
	c.Assert(port, Equals, expected, Commentf("expect port %s", expected))

	port, err = getFreePort("127.0.0.1", expected)
	c.Assert(err, IsNil)
	c.Assert(port == expected, IsFalse, Commentf("should not return same port twice"))
}
