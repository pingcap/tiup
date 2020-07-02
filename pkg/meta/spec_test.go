package meta

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpec(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-*")
	assert.Nil(t, err)

	spec := NewSpec(dir)
	names, err := spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 0)

	// Should ignore directory with meta file.
	err = os.Mkdir(filepath.Join(dir, "dummy"), 0755)
	assert.Nil(t, err)
	names, err = spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 0)

	type Meta struct {
		A string
		B int
	}
	var meta1 = &Meta{
		A: "a",
		B: 1,
	}
	var meta2 = &Meta{
		A: "b",
		B: 2,
	}

	err = spec.SaveClusterMeta("name1", meta1)
	assert.Nil(t, err)

	err = spec.SaveClusterMeta("name2", meta2)
	assert.Nil(t, err)

	getMeta := new(Meta)
	err = spec.ClusterMetadata("name1", getMeta)
	assert.Nil(t, err)
	assert.Equal(t, meta1, getMeta)

	err = spec.ClusterMetadata("name2", getMeta)
	assert.Nil(t, err)
	assert.Equal(t, meta2, getMeta)

	names, err = spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 2)
	sort.Strings(names)
	assert.Equal(t, "name1", names[0])
	assert.Equal(t, "name2", names[1])
}
