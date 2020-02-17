package utils

import (
	"runtime"
)

// GetPlatform returns the current OS and Arch
func GetPlatform() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}
