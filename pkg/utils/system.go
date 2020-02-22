package utils

import (
	"io"
	"os/exec"
	"runtime"
)

// GetPlatform returns the current OS and Arch
func GetPlatform() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}

// Exec creates a process in background and return the PID of it
func Exec(stdout, stderr *io.Writer, name string, arg ...string) (int, error) {
	c := exec.Command(name, arg...)
	c.Stdout = *stdout
	c.Stderr = *stderr
	err := c.Start()
	return c.Process.Pid, err
}
