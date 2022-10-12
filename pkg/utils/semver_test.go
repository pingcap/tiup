package utils

import (
	. "github.com/pingcap/check"
)

var _ = Suite(&TestSemverSuite{})

type TestSemverSuite struct{}

func (s *TestSemverSuite) TestSemverc(c *C) {
	cases := [][]any{
		{"v0.0.1", "v0.0.1", true},
		{"0.0.1", "v0.0.1", true},
		{"invalid", "vinvalid", false},
		{"", "v", false},
		{"nightly", "nightly", true},
		{"Nightly", "Nightly", true},
	}

	for _, cas := range cases {
		v, e := FmtVer(cas[0].(string))
		c.Assert(v, Equals, cas[1].(string))
		c.Assert(e == nil, Equals, cas[2].(bool))
	}
}

func (s *TestSemverSuite) TestVersion(c *C) {
	c.Assert(Version("").IsValid(), IsFalse)
	c.Assert(Version("v3.0.").IsValid(), IsFalse)
	c.Assert(Version("").IsEmpty(), IsTrue)
	c.Assert(Version("").IsNightly(), IsFalse)
	c.Assert(Version("nightly").IsNightly(), IsTrue)
	c.Assert(Version("v1.2.3").String(), Equals, "v1.2.3")
}

func (s *TestSemverSuite) TestConstraint(c *C) {
	cases := []struct {
		constraint string
		version    string
		match      bool
	}{
		{"^4", "4.1.0", true},
		{"4", "4.0.0", true},
		{"4.0", "4.0.0", true},
		{"~4.0", "4.0.5", true},
		{"4.1.x", "4.1.0", true},
		{"4.1.x", "4.1.5", true},
		{"4.x.0", "4.5.0", true},
		{"4.x.0", "4.5.2", true},
		{"4.x.x", "4.5.2", true},
		{"4.3.2-0", "4.3.2", false},
		{"^1.1.0", "1.1.1", true},
		{"~1.1.0", "1.1.1", true},
		{"~1.1.0", "1.2.0", false},
		{"^1.x.x", "1.1.1", true},
		{"^2.x.x", "1.1.1", false},
		{"^1.x.x", "2.1.1", false},
		{"^1.x.x", "1.1.1-beta1", true},
		{"^1.1.2-alpha", "1.2.1-beta.1", true},
		{"^1.2.x", "1.2.1-beta.1", true},
		{"~1.1.1-beta", "1.1.1-alpha", false},
		{"~1.1.1-beta", "1.1.1-beta.1", true},
		{"~1.1.1-beta", "1.1.1", true},
		{"~1.2.3", "1.2.5", true},
		{"~1.2.3", "1.2.2", false},
		{"~1.2.3", "1.3.2", false},
		{"~1.1.*", "1.2.3", false},
		{"~1.3.0", "2.4.5", false},
		{"^4.0", "5.0.0-rc", false},
		{"^4.0-rc", "5.0.0-rc", false},
		{"4.0.0-rc", "4.0.0-rc", true},
		{"~4.0.0-rc", "4.0.0-rc.1", true},
		{"^4", "v5.0.0-20210408", false},
		{"^4.*.*", "5.0.0-0", false},
		{"5.*.*", "5.0.0-0", false},
		{"^4.0.0-1", "4.0.0-1", true},
		{"4.0.0-1", "4.0.0-1", true},
	}
	for _, cas := range cases {
		cons, err := NewConstraint(cas.constraint)
		c.Assert(err, IsNil)
		c.Assert(cons.Check(cas.version), Equals, cas.match)
	}
}
