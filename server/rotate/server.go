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

package rotate

import (
	"fmt"
	"net"
	"strings"

	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/tui/progress"
)

type statusRender struct {
	mbar *progress.MultiBar
	bars map[string]*progress.MultiBarItem
}

func newStatusRender(manifest *v1manifest.Manifest, addr string) *statusRender {
	ss := strings.Split(addr, ":")
	if strings.Trim(ss[0], " ") == "" || strings.Trim(ss[0], " ") == "0.0.0.0" {
		addrs, _ := net.InterfaceAddrs()
		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() && ip.IP.To4() != nil {
				ss[0] = ip.IP.To4().String()
				break
			}
		}
	}

	status := &statusRender{
		mbar: progress.NewMultiBar(fmt.Sprintf("Waiting all administrators to sign http://%s/rotate/root.json", strings.Join(ss, ":"))),
		bars: make(map[string]*progress.MultiBarItem),
	}
	root := manifest.Signed.(*v1manifest.Root)
	for key := range root.Roles[v1manifest.ManifestTypeRoot].Keys {
		status.bars[key] = status.mbar.AddBar(fmt.Sprintf("  - Waiting key %s", key))
	}
	status.mbar.StartRenderLoop()
	return status
}

func (s *statusRender) render(manifest *v1manifest.Manifest) {
	for _, sig := range manifest.Signatures {
		s.bars[sig.KeyID].UpdateDisplay(&progress.DisplayProps{
			Prefix: fmt.Sprintf("  - Waiting key %s", sig.KeyID),
			Mode:   progress.ModeDone,
		})
	}
}

func (s *statusRender) stop() {
	s.mbar.StopRenderLoop()
}
