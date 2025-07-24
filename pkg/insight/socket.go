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

package insight

import "github.com/vishvananda/netlink"

type Socket struct {
	Family     uint8  `json:"family"`
	State      uint8  `json:"state"`
	SourceAddr string `json:"source_addr"`
	SourcePort uint16 `json:"source_port"`
	DestAddr   string `json:"dest_addr"`
	DestPort   uint16 `json:"dest_port"`
}

func (info *InsightInfo) collectSockets() error {
	sockets, err := GetIPV4Sockets(netlink.TCP_ESTABLISHED)
	info.Sockets = sockets
	return err
}
