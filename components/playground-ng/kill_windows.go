//go:build windows
// +build windows

package main

import "syscall"

func killProcessOrGroup(pid int, sig syscall.Signal) error {
	// Playground-NG only supports Linux/macOS. Keep this as a no-op so the
	// package can still compile in unsupported environments.
	_ = pid
	_ = sig
	return nil
}

