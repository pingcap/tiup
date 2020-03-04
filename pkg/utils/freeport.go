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
	"fmt"
	"net"
	"sync"
	"time"
)

// To avoid the same port be generated twice in a short time
var portCache sync.Map

// GetFreePort asks the kernel for a free open port that is ready to use.
func GetFreePort(priority int) (int, error) {
	if port, err := getPort(priority); err == nil {
		return port, nil
	} else if port, err := getPort(0); err == nil {
		return port, nil
	} else {
		return 0, err
	}
}

// MustGetFreePort asks the kernel for a free open port that is ready to use, if fail, panic
func MustGetFreePort(priority int) int {
	if port, err := GetFreePort(priority); err == nil {
		return port
	}
	panic("can't get a free port")
}

func getPort(port int) (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	port = l.Addr().(*net.TCPAddr).Port
	l.Close()

	if t, ok := portCache.Load(port); ok && t.(time.Time).Add(time.Minute).After(time.Now()) {
		return getPort((port + 1) % 65536)
	}
	portCache.Store(port, time.Now())
	return port, nil
}
