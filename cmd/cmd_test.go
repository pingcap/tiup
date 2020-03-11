package cmd

import (
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/pingcap/check"
)

func TestCMD(t *testing.T) {
	TestingT(t)
}

func currentDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}
