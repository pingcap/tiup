package utils

import (
	"bytes"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.RemoveAll(path.Join(currentDir(), "testdata", "parent"))
	os.RemoveAll(path.Join(currentDir(), "testdata", "ssh-exec"))
	os.RemoveAll(path.Join(currentDir(), "testdata", "nop-nop"))
	m.Run()
}

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func TestIsExist(t *testing.T) {
	require.True(t, IsExist("/tmp"))
	require.False(t, IsExist("/tmp/"+uuid.New().String()))
}

func TestIsNotExist(t *testing.T) {
	require.False(t, IsNotExist("/tmp"))
	require.True(t, IsNotExist("/tmp/"+uuid.New().String()))
}

func TestIsExecBinary(t *testing.T) {
	require.False(t, IsExecBinary("/tmp"))

	e := path.Join(currentDir(), "testdata", "ssh-exec")
	f, err := os.OpenFile(e, os.O_CREATE, 0o777)
	require.NoError(t, err)
	defer f.Close()
	require.True(t, IsExecBinary(e))

	e = path.Join(currentDir(), "testdata", "nop-nop")
	f, err = os.OpenFile(e, os.O_CREATE, 0o666)
	require.NoError(t, err)
	defer f.Close()
	require.False(t, IsExecBinary(e))
}

func TestUntar(t *testing.T) {
	require.True(t, IsNotExist(path.Join(currentDir(), "testdata", "parent")))
	f, err := os.Open(path.Join(currentDir(), "testdata", "test.tar.gz"))
	require.NoError(t, err)
	defer f.Close()
	err = Untar(f, path.Join(currentDir(), "testdata"))
	require.NoError(t, err)
	require.True(t, IsExist(path.Join(currentDir(), "testdata", "parent", "child", "content")))
}

func TestCopy(t *testing.T) {
	require.Error(t, Copy(path.Join(currentDir(), "testdata", "test.tar.gz"), "/tmp/not-exists/test.tar.gz"))
	require.NoError(t, Copy(path.Join(currentDir(), "testdata", "test.tar.gz"), "/tmp/test.tar.gz"))
	fi, err := os.Stat(path.Join(currentDir(), "testdata", "test.tar.gz"))
	require.NoError(t, err)
	fii, err := os.Stat("/tmp/test.tar.gz")
	require.NoError(t, err)
	require.Equal(t, fi.Mode(), fii.Mode())

	require.NoError(t, os.Chmod("/tmp/test.tar.gz", 0o777))
	require.NoError(t, Copy(path.Join(currentDir(), "testdata", "test.tar.gz"), "/tmp/test.tar.gz"))
	fi, err = os.Stat(path.Join(currentDir(), "testdata", "test.tar.gz"))
	require.NoError(t, err)
	fii, err = os.Stat("/tmp/test.tar.gz")
	require.NoError(t, err)
	require.Equal(t, fi.Mode(), fii.Mode())
}

func TestIsSubDir(t *testing.T) {
	paths := [][]string{
		{"a", "a"},
		{"../a", "../a/b"},
		{"a", "a/b"},
		{"/a", "/a/b"},
	}
	for _, p := range paths {
		require.True(t, IsSubDir(p[0], p[1]))
	}

	paths = [][]string{
		{"/a", "a/b"},
		{"/a/b/c", "/a/b"},
		{"/a/b", "/a/b1"},
	}
	for _, p := range paths {
		require.False(t, IsSubDir(p[0], p[1]))
	}
}

func TestSaveFileWithBackup(t *testing.T) {
	dir := t.TempDir()
	name := "meta.yaml"

	for i := range 10 {
		err := SaveFileWithBackup(filepath.Join(dir, name), []byte(strconv.Itoa(i)), "")
		require.NoError(t, err)
	}

	// Verify the saved files.
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "meta") {
			paths = append(paths, path)
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 10, len(paths))

	sort.Strings(paths)
	for i, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(i), string(data))
	}

	// test with specify backup dir
	dir = t.TempDir()
	backupDir := t.TempDir()
	for i := range 10 {
		err := SaveFileWithBackup(filepath.Join(dir, name), []byte(strconv.Itoa(i)), backupDir)
		require.NoError(t, err)
	}
	// Verify the saved files in backupDir.
	paths = nil
	err = filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "meta") {
			paths = append(paths, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 9, len(paths))

	sort.Strings(paths)
	for i, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(i), string(data))
	}

	// Verify the latest saved file.
	data, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	require.Equal(t, "9", string(data))
}

func TestConcurrentSaveFileWithBackup(t *testing.T) {
	dir := t.TempDir()
	name := "meta.yaml"
	data := []byte("concurrent-save-file-with-backup")

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(rand.Intn(100)+4) * time.Millisecond)
			err := SaveFileWithBackup(filepath.Join(dir, name), data, "")
			require.NoError(t, err)
		}()
	}

	wg.Wait()

	// Verify the saved files.
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "meta") {
			paths = append(paths, path)
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 10, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, len(data), len(body))
		require.True(t, bytes.Equal(body, data))
	}
}
