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
	"context"
	"errors"
	"os/exec"
)

// ErrorWaitTimeout is used to represent timeout of a command
// Example:
//		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
//		if err := WaitContext(context.WithTimeout(context.Background(), 3), cmd); err == ErrorWaitTimeout {
//			// Do something
//		}
var ErrorWaitTimeout = errors.New("wait command timeout")

// WaitContext wrap cmd.Wait with context
func WaitContext(ctx context.Context, cmd *exec.Cmd) error {
	// We use cmd.Process.Wait instead of cmd.Wait because cmd.Wait is not reenterable
	c := make(chan error, 1)
	go func() {
		if cmd == nil || cmd.Process == nil {
			c <- nil
		} else {
			_, err := cmd.Process.Wait()
			c <- err
		}
	}()
	select {
	case <-ctx.Done():
		return ErrorWaitTimeout
	case err := <-c:
		return err
	}
}
