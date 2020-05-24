package store

import (
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	. "github.com/pingcap/check"
)

var _ = Suite(&TestQCloudStoreSuite{})

type TestQCloudStoreSuite struct{}

func (s *TestQCloudStoreSuite) TestEmptyCommit(c *C) {
	store := NewStore("/tmp/store")
	txn, err := store.Begin()
	c.Assert(err, IsNil)
	c.Assert(txn.Commit(), IsNil)
}

func (s *TestQCloudStoreSuite) TestSingleWrite(c *C) {
	store := NewStore("/tmp/store")
	txn, err := store.Begin()
	c.Assert(err, IsNil)
	c.Assert(txn.WriteManifest("test.json", &v1manifest.Manifest{}), IsNil)
	c.Assert(txn.ReadManifest("test.json", &v1manifest.Manifest{}), IsNil)
	c.Assert(txn.Commit(), IsNil)
}

func (s *TestQCloudStoreSuite) TestWrite(c *C) {
	store := NewStore("/tmp/store")
	txn1, err := store.Begin()
	c.Assert(err, IsNil)
	txn2, err := store.Begin()
	c.Assert(err, IsNil)
	c.Assert(txn1.WriteManifest("test.json", &v1manifest.Manifest{}), IsNil)
	c.Assert(txn2.WriteManifest("test.json", &v1manifest.Manifest{}), IsNil)
	c.Assert(txn1.Commit(), IsNil)
	c.Assert(txn2.Commit(), NotNil)
}
