package proc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceDatasource(t *testing.T) {
	origin := `
a: ${DS_1-CLUSTER}
b: test-cluster
c: Test-Cluster
d: ${DS_LIGHTNING}
`

	dir, err := os.MkdirTemp("", "play_replace_test_*")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	fname := filepath.Join(dir, "a.json")
	err = os.WriteFile(fname, []byte(origin), 0644)
	assert.Nil(t, err)

	name := "myname"
	err = replaceDatasource(dir, name)
	assert.Nil(t, err)

	data, err := os.ReadFile(fname)
	assert.Nil(t, err)
	replaced := string(data)

	n := strings.Count(replaced, name)
	assert.Equal(t, 4, n, "replaced: %s", replaced)
}
