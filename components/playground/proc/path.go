package proc

import (
	"os"
	"path/filepath"
)

// ResolveSiblingBinary searches for an executable named want near baseBinPath.
//
// It checks the directory containing baseBinPath and up to 3 parent directories.
// The returned bool reports whether the binary exists.
func ResolveSiblingBinary(baseBinPath, want string) (string, bool) {
	dir := filepath.Dir(baseBinPath)
	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, want)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(filepath.Dir(baseBinPath), want), false
}
