package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportVersion(t *testing.T) {
	assert := require.New(t)

	tests := map[string]bool{ // version to support or not
		"v2.0.0":        true,
		"v3.0.0":        true,
		"v2.0.0-beta.1": true,
		"v2.0.0-alpha":  true,
		"v1.0.1":        false,
		"v1.1.1":        false,
	}

	for v, support := range tests {
		err := supportVersion(v)
		if support {
			assert.Nil(err)
		} else {
			assert.NotNil(err)
		}
	}

}
