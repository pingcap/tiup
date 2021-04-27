package utils

import (
	"regexp"

	. "github.com/pingcap/check"
)

var _ = Suite(&TestRegexpSuite{})

type TestRegexpSuite struct{}

func (s *TestRegexpSuite) TestMatchGroups(c *C) {
	cases := []struct {
		re       string
		str      string
		expected map[string]string
	}{
		{
			re:  `^(?P<first>[a-zA-Z]*)(?P<second>[0-9]*)$`,
			str: "abc123",
			expected: map[string]string{
				"":       "abc123",
				"first":  "abc",
				"second": "123",
			},
		},
	}

	for _, cas := range cases {
		c.Assert(
			MatchGroups(regexp.MustCompile(cas.re), cas.str),
			DeepEquals, cas.expected)
	}
}
