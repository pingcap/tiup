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

package instance

import (
	"fmt"
	"io"

	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

const tiflashProxyConfig = `
log-file = "%[1]s/tiflash_tikv.log"

[rocksdb]
wal-dir = ""
max-open-files = 256

[security]
ca-path = ""
cert-path = ""
key-path = ""

[server]
addr = "0.0.0.0:%[4]d"
advertise-addr = "%[2]s:%[4]d"
engine-addr = "%[2]s:%[3]d"
%[5]s

[storage]
data-dir = "%[6]s"

[raftdb]
max-open-files = 256
`

func writeTiFlashProxyConfig(w io.Writer, version utils.Version, host, deployDir string, servicePort, proxyPort, proxyStatusPort int) error {
	// TODO: support multi-dir
	dataDir := fmt.Sprintf("%s/flash", deployDir)
	logDir := fmt.Sprintf("%s/log", deployDir)
	ip := AdvertiseHost(host)
	var statusAddr string
	if semver.Compare(version.String(), "v4.0.5") >= 0 || version.IsNightly() {
		statusAddr = fmt.Sprintf(`status-addr = "0.0.0.0:%[2]d"
advertise-status-addr = "%[1]s:%[2]d"`, ip, proxyStatusPort)
	} else {
		statusAddr = fmt.Sprintf(`status-addr = "%[1]s:%[2]d"`, ip, proxyStatusPort)
	}
	conf := fmt.Sprintf(tiflashProxyConfig, logDir, ip, servicePort, proxyPort, statusAddr, dataDir)
	_, err := w.Write([]byte(conf))
	return err
}
