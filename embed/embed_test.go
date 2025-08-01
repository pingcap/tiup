package embed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func getAllFilePaths(dir string) (paths []string, err error) {
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if path == dir {
			return nil
		}
		if info.IsDir() {
			subPaths, err := getAllFilePaths(path)
			if err != nil {
				return err
			}
			paths = append(paths, subPaths...)
		} else {
			paths = append(paths, path)
		}

		return nil
	})

	return
}

// Test can read all file in /templates
func TestCanReadTemplates(t *testing.T) {
	paths, err := getAllFilePaths("templates")
	require.Nil(t, err)
	require.Greater(t, len(paths), 0)

	for _, path := range paths {
		t.Log("check file: ", path)

		data, err := os.ReadFile(path)
		require.Nil(t, err)

		embedData, err := ReadTemplate(path)
		require.Nil(t, err)

		require.Equal(t, embedData, data)
	}
}

// Test can read all file in /examples
func TestCanReadExamples(t *testing.T) {
	paths, err := getAllFilePaths("examples")
	require.Nil(t, err)
	require.Greater(t, len(paths), 0)

	for _, path := range paths {
		t.Log("check file: ", path)

		data, err := os.ReadFile(path)
		require.Nil(t, err)

		embedData, err := ReadExample(path)
		require.Nil(t, err)

		require.Equal(t, embedData, data)
	}
}
