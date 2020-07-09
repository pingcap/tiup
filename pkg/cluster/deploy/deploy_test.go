package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCompare(t *testing.T) {
	var err error

	err = versionCompare("v4.0.0", "v4.0.1")
	assert.Nil(t, err)

	err = versionCompare("v4.0.1", "v4.0.0")
	assert.NotNil(t, err)

	err = versionCompare("v4.0.0", "nightly")
	assert.Nil(t, err)

	err = versionCompare("nightly", "nightly")
	assert.Nil(t, err)
}
