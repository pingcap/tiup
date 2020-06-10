package main

import (
	"io/ioutil"
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

	dir, err := ioutil.TempDir("", "play_replace_test_*")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	fname := filepath.Join(dir, "a.json")
	err = ioutil.WriteFile(fname, []byte(origin), 0644)
	assert.Nil(t, err)

	name := "myname"
	err = replaceDatasource(dir, name)
	assert.Nil(t, err)

	data, err := ioutil.ReadFile(fname)
	assert.Nil(t, err)
	replaced := string(data)

	n := strings.Count(replaced, name)
	assert.Equal(t, 4, n, "replaced: %s", replaced)
}
