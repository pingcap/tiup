package utils

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchGroups(t *testing.T) {
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
		require.Equal(t, cas.expected, MatchGroups(regexp.MustCompile(cas.re), cas.str))
	}
}
