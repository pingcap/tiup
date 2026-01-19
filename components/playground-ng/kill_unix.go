//go:build !windows
// +build !windows

package main

import "syscall"

func killProcessOrGroup(pid int, sig syscall.Signal) error {
	if pid <= 0 || sig == 0 {
		return nil
	}

	// On Linux, playground-ng starts processes with Setpgid=true, so pgid==pid
	// and we can safely kill the whole process group to avoid leaving detached
	// children behind on force kill.
	//
	// On other Unix platforms, this is best-effort: only kill the group when the
	// process is the group leader, otherwise fall back to pid to avoid
	// accidentally killing unrelated processes.
	if pgid, err := syscall.Getpgid(pid); err == nil && pgid == pid {
		return syscall.Kill(-pid, sig)
	}
	return syscall.Kill(pid, sig)
}
