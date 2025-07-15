// Check if epoll exclusive available on the host
// Ported from https://github.com/pingcap/tidb-ansible/blob/v3.1.0/scripts/check/epoll_chk.cc

//go:build cgo && linux
// +build cgo,linux

package insight

/*
#include <sys/eventfd.h>
*/
import "C"

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// checkEpollExclusive checks if the host system support epoll exclusive mode
func checkEpollExclusive() bool {
	fd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil || fd < 0 {
		return false
	}
	defer syscall.Close(fd)

	evfd, err := C.eventfd(0, C.EFD_NONBLOCK|C.EFD_CLOEXEC)
	if err != nil || evfd < 0 {
		return false
	}
	defer syscall.Close(int(evfd))

	/* choose events that should cause an error on
	   EPOLLEXCLUSIVE enabled kernels - specifically the combination of
	   EPOLLONESHOT and EPOLLEXCLUSIVE */
	ev := syscall.EpollEvent{
		Events: unix.EPOLLET |
			unix.EPOLLIN |
			unix.EPOLLEXCLUSIVE |
			unix.EPOLLONESHOT,
		//Fd: int32(fd),
	}
	if err := syscall.EpollCtl(fd, unix.EPOLL_CTL_ADD, int(evfd), &ev); err != nil {
		if err != syscall.EINVAL {
			return false
		} // else true
	} else {
		return false
	}

	return true
}
