package command

import (
	"testing"

	"github.com/pingcap/check"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

type upgradeSuite struct{}

var _ = check.Suite(&upgradeSuite{})

func (s *upgradeSuite) TestVersionCompare(c *check.C) {
	var err error

	err = versionCompare("v4.0.0", "v4.0.1")
	c.Assert(err, check.IsNil)

	err = versionCompare("v4.0.1", "v4.0.0")
	c.Assert(err, check.NotNil)

	err = versionCompare("v4.0.0", "nightly")
	c.Assert(err, check.IsNil)

	err = versionCompare("nightly", "nightly")
	c.Assert(err, check.IsNil)
}
