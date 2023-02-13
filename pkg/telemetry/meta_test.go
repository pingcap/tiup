package telemetry

import (
	"os"

	"github.com/pingcap/check"
)

type teleSuite struct{}

var _ = check.Suite(&teleSuite{})

func (s *teleSuite) TestTelemetry(c *check.C) {
	// get a temp file and remove it.
	file, err := os.CreateTemp("", "")
	c.Assert(err, check.IsNil)

	fname := file.Name()
	file.Close()
	err = os.Remove(fname)
	c.Assert(err, check.IsNil)

	// Should no error and get a default meta.
	meta, err := LoadFrom(fname)
	c.Assert(err, check.IsNil)
	c.Assert(meta.Status, check.Equals, Status(""))
	c.Assert(len(meta.UUID), check.Equals, 0)

	// Save and load back
	err = meta.SaveTo(fname)
	c.Assert(err, check.IsNil)

	var meta2 *Meta
	meta2, err = LoadFrom(fname)
	c.Assert(err, check.IsNil)
	c.Assert(meta2, check.DeepEquals, meta)

	// Update UUID
	meta2.UUID = NewUUID()
	err = meta2.SaveTo(fname)
	c.Assert(err, check.IsNil)

	var meta3 *Meta
	meta3, err = LoadFrom(fname)
	c.Assert(err, check.IsNil)
	c.Assert(meta3, check.DeepEquals, meta2)
}
