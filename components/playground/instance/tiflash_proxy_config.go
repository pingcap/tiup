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

	"github.com/pingcap/tiup/pkg/repository/v0manifest"
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
status-addr = "%[2]s:%[5]d"

[storage]
data-dir = "%[6]s"

[raftdb]
max-open-files = 256
`

const tiflashProxyConfigV405 = `
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
status-addr = "0.0.0.0:%[5]d"
advertise-status-addr = "%[2]s:%[5]d"

[storage]
data-dir = "%[6]s"

[raftdb]
max-open-files = 256
`

func writeTiFlashProxyConfig(w io.Writer, version v0manifest.Version, ip, deployDir string, servicePort, proxyPort, proxyStatusPort int) error {
	// TODO: support multi-dir
	dataDir := fmt.Sprintf("%s/flash", deployDir)
	logDir := fmt.Sprintf("%s/log", deployDir)
	var conf string
	if semver.Compare("v4.0.5", version.String()) <= 0 {
		conf = fmt.Sprintf(tiflashProxyConfigV405, logDir, ip, servicePort, proxyPort, proxyStatusPort, dataDir)
	} else {
		conf = fmt.Sprintf(tiflashProxyConfig, logDir, ip, servicePort, proxyPort, proxyStatusPort, dataDir)
	}
	_, err := w.Write([]byte(conf))
	return err
}
