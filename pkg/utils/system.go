// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"io"
	"os/exec"
)

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
