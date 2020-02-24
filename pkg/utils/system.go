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
func Exec(
	stdout, stderr *io.Writer,
	dir string,
	name string,
	arg ...string) (int, error) {

	// init the command
	c := exec.Command(name, arg...)
	if stdout != nil {
		c.Stdout = *stdout
	}
	if stderr != nil {
		c.Stderr = *stderr
	}
	if dir != "" {
		c.Dir = dir
	}
	err := c.Start()
	return c.Process.Pid, err
}
