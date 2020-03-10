package cmd

import (
	"os"
	"path"
	"path/filepath"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	. "github.com/pingcap/check"
)

var _ = Suite(&TestListSuite{})

type TestListSuite struct {
	mirror  meta.Mirror
	testDir string
}

func (s *TestListSuite) SetUpSuite(c *C) {
	s.testDir = filepath.Join(currentDir(), "testdata")
	s.mirror = meta.NewMirror(s.testDir)
	c.Assert(s.mirror.Open(), IsNil)
	repository = meta.NewRepository(s.mirror, meta.RepositoryOptions{})
	os.RemoveAll(path.Join(s.testDir, "profile"))
	os.MkdirAll(path.Join(s.testDir, "profile"), 0755)
	profile = localdata.NewProfile(path.Join(s.testDir, "profile"))
}

func (s *TestListSuite) TearDownSuite(c *C) {
	s.mirror.Close()
	os.RemoveAll(path.Join(s.testDir, "profile"))
}

func (s *TestListSuite) TestListComponent(c *C) {
	cmd := newListCmd()

	//c.Assert(utils.IsNotExist(path.Join(s.testDir, "profile", "components", "test")), IsTrue)
	c.Assert(cmd.RunE(cmd, []string{}), IsNil)
	//c.Assert(utils.IsExist(path.Join(s.testDir, "profile", "components", "test")), IsTrue)
}
