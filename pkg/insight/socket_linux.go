// Copyright 2021 PingCAP, Inc.
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

//go:build linux
// +build linux

package insight

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// GetIPV4Sockets is getting sockets from states
func GetIPV4Sockets(states ...uint8) ([]Socket, error) {
	netSockets, err := netlink.SocketDiagTCP(unix.AF_INET)
	if err != nil {
		return nil, err
	}

	tcpStates := make(map[uint8]bool, len(states))
	for _, state := range states {
		tcpStates[state] = true
	}
	sockets := make([]Socket, 0, len(netSockets))
	for _, socket := range netSockets {
		if len(tcpStates) > 0 && !tcpStates[socket.State] {
			continue
		}
		sockets = append(sockets, Socket{
			Family:     socket.Family,
			State:      socket.State,
			SourceAddr: socket.ID.Source.String(),
			SourcePort: socket.ID.SourcePort,
			DestAddr:   socket.ID.Destination.String(),
			DestPort:   socket.ID.DestinationPort,
		})
	}

	return sockets, nil
}
