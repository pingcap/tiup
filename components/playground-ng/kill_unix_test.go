//go:build !windows
// +build !windows

package main

import (
	"bufio"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKillProcessOrGroup_KillsChildWhenLeader(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 1000 & echo $!; wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = io.Discard

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)

	require.NoError(t, cmd.Start())
	parentPID := cmd.Process.Pid
	t.Cleanup(func() {
		_ = killProcessOrGroup(parentPID, syscall.SIGKILL)
		_ = cmd.Process.Kill()
	})

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	require.NoError(t, err)
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	require.NoError(t, err)
	require.Greater(t, childPID, 0)

	// Validate our assumption: the parent is the process group leader.
	pgid, err := syscall.Getpgid(parentPID)
	require.NoError(t, err)
	require.Equal(t, parentPID, pgid)

	// Ensure the child exists before killing.
	require.NoError(t, syscall.Kill(childPID, 0))

	_ = killProcessOrGroup(parentPID, syscall.SIGKILL)
	_ = cmd.Wait()

	require.Eventually(t, func() bool {
		return syscall.Kill(childPID, 0) != nil
	}, 2*time.Second, 20*time.Millisecond)
}
